package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/oauth"
)

// SMTPSender sends mail through the configured account.
type SMTPSender struct {
	config config.SMTPConfig
	from   string
}

// NewSMTPSender returns a sender for the account; from is kept for callers
// that want to show it.
func NewSMTPSender(cfg config.SMTPConfig, from string) *SMTPSender {
	return &SMTPSender{config: cfg, from: from}
}

// Send delivers msg and reports the Message-ID it went out under.
func (s *SMTPSender) Send(ctx context.Context, msg Message) Result {
	fail := func(err error) Result { return Result{Success: false, Error: err} }
	if err := validateMessage(msg); err != nil {
		return fail(err)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") { // header injection
		return fail(fmt.Errorf("subject contains invalid characters"))
	}

	auth, err := s.authenticator(ctx)
	if err != nil {
		return fail(err)
	}
	if auth == nil && s.config.Username != "" {
		return fail(fmt.Errorf("SMTP auth requires TLS"))
	}

	// our own Message-ID, so the one recorded in history is the one the
	// broker receives and quotes back in In-Reply-To
	domain := "localhost"
	if from, err := mail.ParseAddress(msg.From); err == nil {
		domain = from.Address[strings.LastIndex(from.Address, "@")+1:]
	}
	messageID := "<" + rand.Text() + "@" + domain + ">"

	raw := []byte(formatMessage(msg, messageID, time.Now()))
	if err := s.transact(ctx, auth, msg.From, msg.To, raw, s.config.TLS()); err != nil {
		return fail(sanitizeSMTPError(err))
	}
	return Result{Success: true, MessageID: messageID}
}

// authenticator is the login for the configured account: a password or an
// OAuth token. It is nil when the connection is not TLS, since credentials
// must never cross in plaintext.
func (s *SMTPSender) authenticator(ctx context.Context) (smtp.Auth, error) {
	if !s.config.TLS() {
		return nil, nil
	}
	secret, err := config.Reveal(s.config.Password)
	if err != nil {
		return nil, err
	}
	if s.config.Auth != config.AuthOAuth2 {
		return smtp.PlainAuth("", s.config.Username, secret, s.config.Host), nil
	}
	token, err := oauth.AccessFor(ctx, config.OAuthClientID(s.config.ClientID), secret)
	if err != nil {
		return nil, err
	}
	return oauth.SMTPAuth(s.config.Username, token), nil
}

// formatMessage builds the RFC 5322 message. A non-ASCII subject is encoded
// as an RFC 2047 word, and a body that is not plain 7-bit text (accented
// names, very long lines) is sent quoted-printable, so no server on the way
// has to support 8-bit mail. Date is required by the RFC and its absence
// counts against a message in spam scoring.
func formatMessage(msg Message, messageID string, now time.Time) string {
	var b strings.Builder
	header := func(name, value string) { b.WriteString(name + ": " + value + "\r\n") }
	header("Message-ID", messageID)
	header("Date", now.Format(time.RFC1123Z))
	header("From", msg.From)
	header("To", msg.To)
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	if plainEnough(msg.Body) {
		header("Content-Transfer-Encoding", "7bit")
		b.WriteString("\r\n" + msg.Body)
		return b.String()
	}
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	w := quotedprintable.NewWriter(&b)
	_, _ = w.Write([]byte(msg.Body))
	_ = w.Close()
	return b.String()
}

// plainEnough reports whether text can go out as 7bit: ASCII only, lines no
// longer than the 998 octets RFC 5322 allows.
func plainEnough(text string) bool {
	line := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 0x80 || c == 0 {
			return false
		}
		if c == '\n' {
			line = 0
			continue
		}
		if line++; line > 998 {
			return false
		}
	}
	return true
}

// smtpErrors turns a low-level error into a message that is safe to store and
// show: no server banners, hostnames or credentials. The first matching
// fragment wins.
var smtpErrors = []struct {
	fragments []string
	message   string
}{
	{[]string{"auth"}, "SMTP authentication failed"},
	{[]string{"certificate"}, "TLS certificate error"},
	// "actively refused" is how Windows words a refused connection.
	{[]string{"connection refused", "actively refused"}, "could not connect to the SMTP server (wrong host/port, or a local bridge like Proton Mail Bridge isn't running)"},
	{[]string{"starttls"}, "SMTP server does not support STARTTLS on this port"},
}

func sanitizeSMTPError(err error) error {
	text := strings.ToLower(err.Error())
	for _, rule := range smtpErrors {
		for _, f := range rule.fragments {
			if strings.Contains(text, f) {
				return errors.New(rule.message)
			}
		}
	}
	return errors.New("SMTP error: check your configuration")
}

func (s *SMTPSender) address() string {
	return net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
}

// connect dials the server and secures the connection: TLS from the first byte
// on port 465, mandatory STARTTLS elsewhere, never opportunistic, because the
// login must not cross in plaintext. net/smtp ignores contexts, so a silent
// server would hang forever; closing the connection when ctx ends unblocks its
// reads and writes. Call stop when finished.
func (s *SMTPSender) connect(ctx context.Context, useTLS bool) (*smtp.Client, func(), error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", s.address())
	if err != nil {
		return nil, nil, fmt.Errorf("connection failed: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-finished:
		}
	}()
	stop := func() { close(finished) }
	abort := func(c io.Closer, err error) (*smtp.Client, func(), error) {
		_ = c.Close()
		stop()
		return nil, nil, err
	}

	wire := net.Conn(conn)
	implicitTLS := useTLS && config.ImplicitTLS(s.config.Port)
	if implicitTLS {
		secured := tls.Client(conn, config.TLSFor(s.config.Host))
		if err := secured.HandshakeContext(ctx); err != nil {
			return abort(conn, fmt.Errorf("TLS handshake failed: %w", err))
		}
		wire = secured
	}
	client, err := smtp.NewClient(wire, s.config.Host)
	if err != nil {
		return abort(wire, fmt.Errorf("SMTP client creation failed: %w", err))
	}
	if useTLS && !implicitTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return abort(client, fmt.Errorf("server does not offer STARTTLS on port %d", s.config.Port))
		}
		if err := client.StartTLS(config.TLSFor(s.config.Host)); err != nil {
			return abort(client, fmt.Errorf("STARTTLS failed: %w", err))
		}
	}
	return client, stop, nil
}

// VerifyLogin connects and authenticates without sending anything, so setup can
// tell a bad login from a working one. The error keeps the server's own words
// (Send's is sanitized) because it is shown to the person who just set the
// account up.
func (s *SMTPSender) VerifyLogin(ctx context.Context) error {
	if !s.config.TLS() {
		return errors.New("SMTP login needs TLS")
	}
	auth, err := s.authenticator(ctx)
	if err != nil {
		return err
	}
	client, stop, err := s.connect(ctx, true)
	if err != nil {
		return err
	}
	defer stop()
	defer func() { _ = client.Close() }()
	if err := client.Auth(auth); err != nil {
		return err
	}
	return client.Quit()
}

// transact runs one SMTP transaction under ctx: login if auth is set, then
// MAIL FROM, RCPT TO and DATA.
func (s *SMTPSender) transact(ctx context.Context, auth smtp.Auth, from, to string, data []byte, useTLS bool) error {
	client, stop, err := s.connect(ctx, useTLS)
	if err != nil {
		return err
	}
	defer stop()
	defer func() { _ = client.Close() }()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}
	steps := []struct {
		what string
		run  func() error
	}{
		{"sender rejected", func() error { return client.Mail(from) }},
		{"recipient rejected", func() error { return client.Rcpt(to) }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.what, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data command failed: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("message finalization failed: %w", err)
	}
	return client.Quit()
}
