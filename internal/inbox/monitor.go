package inbox

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/oauth"
	"github.com/emersion/go-imap/client"
)

// Monitor reads the inbox over IMAP and recognises mail from known brokers.
type Monitor struct {
	config  config.InboxConfig
	client  *client.Client
	brokers map[string]broker.Broker // by the domain mail comes from; by full address for a free-mail broker
}

// freeMail domains host unrelated senders, so a broker on one (a gmail.com
// privacy address) owns its address, never the domain.
var freeMail = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true, "ymail.com": true,
	"outlook.com": true, "hotmail.com": true, "live.com": true, "msn.com": true,
	"aol.com": true, "icloud.com": true, "me.com": true, "mac.com": true,
	"protonmail.com": true, "proton.me": true, "gmx.com": true, "gmx.net": true,
	"mail.com": true, "zoho.com": true, "yandex.com": true,
}

// brokerFor finds the broker a sender belongs to.
func (m *Monitor) brokerFor(from, domain string) (broker.Broker, bool) {
	if domain == "" {
		return broker.Broker{}, false
	}
	if freeMail[domain] {
		b, ok := m.brokers[strings.ToLower(from)]
		return b, ok
	}
	b, ok := m.brokers[domain]
	return b, ok
}

type Email struct {
	UID        uint32 // the IMAP UID, for moving or deleting it afterwards
	MessageID  string
	From       string
	FromName   string // the sender's display name, e.g. "Mail Delivery System"
	FromDomain string
	Subject    string
	Body       string
	HTMLBody   string
	ReceivedAt time.Time
	BrokerID   string // set when the sender is a known broker
	BrokerName string
}

func NewMonitor(cfg config.InboxConfig, brokerList []broker.Broker) *Monitor {
	byDomain := map[string]broker.Broker{}
	for _, b := range brokerList {
		if _, host, ok := strings.Cut(b.Email, "@"); ok && !strings.Contains(host, "@") {
			if host = strings.ToLower(host); freeMail[host] {
				byDomain[strings.ToLower(strings.TrimSpace(b.Email))] = b
			} else {
				byDomain[host] = b
			}
		}
		if d := b.Domain(); d != "" {
			byDomain[d] = b // the website's domain wins over the mail domain when they are the same
		}
	}
	return &Monitor{config: cfg, brokers: byDomain}
}

// Connect opens the connection and logs in, with a password or OAuth.
func (m *Monitor) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", m.config.Server, m.config.Port)
	log.Printf("Connecting to IMAP server %s...", addr)
	c, err := dialIMAP(addr, m.config.Server, m.config.Port)
	if err != nil {
		return err
	}
	log.Printf("Connected, logging in as %s...", m.config.Email)

	if err := m.login(ctx, c); err != nil {
		_ = c.Logout()
		return err
	}
	m.client = c
	log.Printf("Login successful")
	return nil
}

func (m *Monitor) login(ctx context.Context, c *client.Client) error {
	secret, err := config.Reveal(m.config.Password)
	if err != nil {
		return err
	}
	if m.config.Auth != config.AuthOAuth2 {
		if err := c.Login(m.config.Email, secret); err != nil {
			return fmt.Errorf("failed to login: %w", err)
		}
		return nil
	}
	token, err := oauth.AccessFor(ctx, config.OAuthClientID(m.config.ClientID), secret)
	if err != nil {
		return err
	}
	if err := c.Authenticate(oauth.SASL{User: m.config.Email, Token: token}); err != nil {
		return fmt.Errorf("failed to login with OAuth: %w", err)
	}
	return nil
}

// dialIMAP uses TLS from the first byte on the implicit-TLS port and STARTTLS
// on any other, and never falls back to plaintext: LOGIN sends the password.
func dialIMAP(addr, host string, port int) (*client.Client, error) {
	tlsCfg := config.TLSFor(host)
	if config.ImplicitTLS(port) {
		c, err := client.DialTLS(addr, tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
		}
		return c, nil
	}
	c, err := client.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}
	if ok, err := c.SupportStartTLS(); err != nil || !ok {
		_ = c.Logout()
		return nil, fmt.Errorf("IMAP server on port %d does not offer STARTTLS", port)
	}
	if err := c.StartTLS(tlsCfg); err != nil {
		_ = c.Logout()
		return nil, fmt.Errorf("IMAP STARTTLS failed: %w", err)
	}
	return c, nil
}

func (m *Monitor) Disconnect() error {
	if m.client == nil {
		return nil
	}
	return m.client.Logout()
}

var errNotConnected = fmt.Errorf("not connected to IMAP server")
