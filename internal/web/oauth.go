package web

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/email"
	"github.com/drumandbytes/delistd/internal/inbox"
	"github.com/drumandbytes/delistd/internal/oauth"
)

// signIn is one Microsoft device-code sign-in started from the mail form.
// The refresh token never goes to the browser: the form posts only the id.
type signIn struct {
	clientID string
	user     string // the login the sign-in was verified for
	code     oauth.DeviceCode
	token    string
	err      string
	done     bool
	started  time.Time
}

type signInStore struct {
	mu sync.Mutex
	m  map[string]*signIn
}

// signIns is package-level because readMailForm (which turns the posted id
// into a token) has no Server; Delistd runs one web server per process.
var signIns = &signInStore{m: map[string]*signIn{}}

const signInTTL = 30 * time.Minute

func (st *signInStore) add(s *signIn) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	for id, old := range st.m {
		if time.Since(old.started) > signInTTL {
			delete(st.m, id)
		}
	}
	id := rand.Text()
	st.m[id] = s
	return id
}

func (st *signInStore) get(id string) (signIn, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	s, ok := st.m[id]
	if !ok {
		return signIn{}, false
	}
	return *s, true
}

func (st *signInStore) finish(id, token, errMsg string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if s, ok := st.m[id]; ok {
		s.token, s.err, s.done = token, errMsg, true
	}
}

// token is the refresh token of a finished sign-in started for clientID.
func (st *signInStore) token(id, clientID, user string) (string, bool) {
	s, ok := st.get(id)
	if !ok || !s.done || s.token == "" || s.clientID != clientID || !strings.EqualFold(s.user, user) {
		return "", false
	}
	return s.token, true
}

// signInCheckError says which login failed: "inbox" (IMAP) or "sending" (SMTP).
type signInCheckError struct {
	Stage string
	Err   error
}

func (e signInCheckError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e signInCheckError) Unwrap() error { return e.Err }

// Microsoft needs a few seconds to pass a fresh grant to its mail servers and
// answers "authenticated but not connected" until then.
var (
	verifyRetries    = 6
	verifyRetryDelay = 4 * time.Second
)

// verifySignIn logs in to the inbox and to SMTP exactly as a real run would,
// so a wrong address, a mailbox without IMAP, or SMTP's stricter rule that
// only the account's primary address may log in show up now. Tests replace it.
var verifySignIn = func(ctx context.Context, in config.InboxConfig, out config.SMTPConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < verifyRetries; attempt++ {
		m := inbox.NewMonitor(in, nil)
		if err = m.Connect(ctx); err == nil {
			_ = m.Disconnect()
			break
		}
		if !strings.Contains(err.Error(), "not connected") {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(verifyRetryDelay):
		}
	}
	if err != nil {
		return signInCheckError{"inbox", err}
	}
	if err := email.NewSMTPSender(out, "").VerifyLogin(ctx); err != nil {
		return signInCheckError{"sending", err}
	}
	return nil
}

func signInFailureMessage(user string, err error) string {
	var ce signInCheckError
	if errors.As(err, &ce) && ce.Stage == "sending" {
		return "Microsoft signed you in and the inbox works, but sending as " + user + " was refused (" + ce.Err.Error() + "). Outlook only lets the account's primary address log in to send. If that differs from " + user + " (for example a Gmail-based Microsoft account with an Outlook alias), open Server settings below, enter the primary address as the login username, keep " + user + " as the email address, and sign in again."
	}
	return "Microsoft signed you in, but " + user + " could not log in to its inbox with it (" + err.Error() + "). Check that you approved the sign-in with exactly that account and that its mailbox allows IMAP, or change the address above."
}

// oauthView is partials/oauth-status.html's data.
type oauthView struct {
	ID       string
	Code     string
	URL      string
	User     string
	Pending  bool
	Done     bool
	Error    string
	ClientID string
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	limitFormBody(w, r)
	clientID := strings.TrimSpace(r.FormValue("oauth_client_id")) // "" = the shared default
	if config.OAuthClientID(clientID) == "" {
		s.renderPartial(w, "partials/oauth-status.html", oauthView{Error: "Paste your app's Application (client) ID first (step 5 above)."})
		return
	}
	user := strings.TrimSpace(r.FormValue("mail_username"))
	if user == "" {
		user = strings.TrimSpace(r.FormValue("mail_address"))
	}
	if user == "" {
		s.renderPartial(w, "partials/oauth-status.html", oauthView{Error: "Enter your Outlook.com address above first: Delistd checks that the sign-in belongs to it."})
		return
	}
	c := oauth.NewClient(oauth.MicrosoftConsumers, config.OAuthClientID(clientID))
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	dc, err := c.StartDeviceCode(ctx)
	if err != nil {
		s.renderPartial(w, "partials/oauth-status.html", oauthView{Error: err.Error()})
		return
	}
	id := signIns.add(&signIn{clientID: clientID, user: user, code: dc, started: time.Now()})
	go func() {
		pctx, stop := context.WithTimeout(context.Background(), time.Duration(dc.ExpiresIn)*time.Second+30*time.Second)
		defer stop()
		tok, err := c.PollDeviceCode(pctx, dc)
		switch {
		case err != nil:
			signIns.finish(id, "", err.Error())
		case tok.Refresh == "":
			signIns.finish(id, "", "Microsoft returned no refresh token; check the app allows offline_access (see the guide).")
		default:
			p := config.OutlookOAuth
			if err := verifySignIn(pctx,
				config.InboxConfig{Server: p.IMAPHost, Port: p.IMAPPort, Email: user, Password: tok.Refresh, Auth: config.AuthOAuth2, ClientID: clientID},
				config.SMTPConfig{Host: p.SMTPHost, Port: p.SMTPPort, Username: user, Password: tok.Refresh, Auth: config.AuthOAuth2, ClientID: clientID},
			); err != nil {
				signIns.finish(id, "", signInFailureMessage(user, err))
				return
			}
			signIns.finish(id, tok.Refresh, "")
		}
	}()
	s.renderPartial(w, "partials/oauth-status.html", signInView(id, signIn{clientID: clientID, user: user, code: dc}))
}

func (s *Server) handleOAuthStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	si, ok := signIns.get(id)
	if !ok {
		s.renderPartial(w, "partials/oauth-status.html", oauthView{Error: "This sign-in expired. Start it again."})
		return
	}
	s.renderPartial(w, "partials/oauth-status.html", signInView(id, si))
}

func signInView(id string, si signIn) oauthView {
	v := oauthView{ID: id, Code: si.code.UserCode, URL: si.code.VerificationURI, ClientID: si.clientID, User: si.user}
	switch {
	case si.err != "":
		v.ID, v.Error = "", si.err
	case si.done:
		v.Done = true
	default:
		v.Pending = true
	}
	return v
}
