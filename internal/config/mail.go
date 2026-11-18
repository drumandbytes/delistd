package config

import (
	"fmt"
	"strings"
)

// MailConfig holds one profile's email/inbox overrides. Either field may be
// set independently - e.g. a profile can send from its own SMTP account
// while still sharing the default inbox for reply monitoring.
type MailConfig struct {
	Email *EmailConfig `yaml:"email,omitempty"`
	Inbox *InboxConfig `yaml:"inbox,omitempty"`
}

// EmailForProfile returns the SMTP config to use for sends made under this
// profile: its Mail.Email override if set, otherwise the shared top-level
// Email block.
func (c *Config) EmailForProfile(p NamedProfile) EmailConfig {
	if p.Mail != nil && p.Mail.Email != nil {
		return *p.Mail.Email
	}
	return c.Email
}

// InboxForProfile returns the IMAP config to use when monitoring replies for
// this profile: its Mail.Inbox override if set, otherwise the shared
// top-level Inbox block.
func (c *Config) InboxForProfile(p NamedProfile) InboxConfig {
	if p.Mail != nil && p.Mail.Inbox != nil {
		return *p.Mail.Inbox
	}
	return c.Inbox
}

// ConfiguredInboxes returns every enabled IMAP inbox across profiles (overrides
// or the shared one), deduplicated by address, so `monitor` covers everyone.
func (c *Config) ConfiguredInboxes() []InboxConfig {
	seen := make(map[string]bool)
	var result []InboxConfig
	for _, p := range c.GetProfiles() {
		inbox := c.InboxForProfile(p)
		if !inbox.Enabled {
			continue
		}
		key := strings.ToLower(inbox.Email)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, inbox)
	}
	return result
}

type InboxConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Provider      string `yaml:"provider,omitempty"`  // preset id (see Providers) that fills Server/Port; optional when they're set
	Server        string `yaml:"server"`              // e.g., "imap.fastmail.com"
	Port          int    `yaml:"port"`                // 993 = implicit TLS, anything else = STARTTLS
	Email         string `yaml:"email"`               // Email address to monitor
	Password      string `yaml:"password"`            // App password (not main password); with auth: oauth2, the refresh token
	Auth          string `yaml:"auth,omitempty"`      // "oauth2" signs in with OAuth (see AuthOAuth2); empty = password
	ClientID      string `yaml:"client_id,omitempty"` // OAuth app (client) id; empty = the shared default (DefaultOAuthClientID)
	Folder        string `yaml:"folder"`              // Folder to monitor (default: "INBOX")
	AutoArchive   bool   `yaml:"auto_archive"`        // Automatically move processed emails to archive folder
	ArchiveFolder string `yaml:"archive_folder"`      // Folder to archive emails to (default: "Delistd")
}

type EmailConfig struct {
	From string     `yaml:"from"`
	SMTP SMTPConfig `yaml:"smtp,omitempty"`
}

// Configured reports whether a sending account is set up at all.
func (e EmailConfig) Configured() bool {
	return e.SMTP.Host != ""
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"` // with auth: oauth2, the refresh token
	// Auth is "oauth2" for providers that refuse passwords (Outlook.com); the
	// refresh token then lives in Password and ClientID names the OAuth app.
	Auth     string `yaml:"auth,omitempty"`
	ClientID string `yaml:"client_id,omitempty"`
	// UseTLS defaults to true; the mode follows the port (465 implicit TLS,
	// anything else STARTTLS). false is only for an unauthenticated local
	// relay. Read it via TLS().
	UseTLS *bool `yaml:"use_tls,omitempty"`
}

// TLS returns the effective use_tls, true unless explicitly turned off.
func (s SMTPConfig) TLS() bool {
	return s.UseTLS == nil || *s.UseTLS
}

// ApplyInboxDefaults fills Folder/ArchiveFolder and, for known providers,
// Server/Port. Shared by the top-level inbox, per-profile overrides and configs edited at runtime.
func ApplyInboxDefaults(inbox *InboxConfig) {
	if inbox.Folder == "" {
		inbox.Folder = "INBOX"
	}
	if inbox.ArchiveFolder == "" {
		inbox.ArchiveFolder = "Delistd"
	}
	if inbox.Server != "" {
		return
	}
	if p, ok := ProviderByID(inbox.Provider); ok && p.IMAPHost != "" {
		inbox.Server, inbox.Port = p.IMAPHost, p.IMAPPort
	} else if inbox.Provider == "outlook" { // pre-preset configs; Microsoft has since dropped basic auth
		inbox.Server, inbox.Port = "outlook.office365.com", 993
	}
}

func validateEmailConfig(e EmailConfig) error {
	if !e.Configured() {
		return fmt.Errorf("email.smtp: host is required (or set options.send_mode: manual)")
	}
	if e.From == "" {
		return fmt.Errorf("email: from address is required")
	}
	if e.SMTP.Port == 0 {
		return fmt.Errorf("email.smtp: port is required")
	}
	if e.SMTP.Auth != "" && e.SMTP.Auth != AuthOAuth2 {
		return fmt.Errorf("email.smtp: unknown auth %q (use oauth2 or leave empty)", e.SMTP.Auth)
	}
	if e.SMTP.Auth == AuthOAuth2 {
		if OAuthClientID(e.SMTP.ClientID) == "" {
			return fmt.Errorf("email.smtp: client_id is required with auth: oauth2")
		}
		if e.SMTP.Password == "" {
			return fmt.Errorf("email.smtp: not signed in yet: run `delistd oauth login`")
		}
	}
	if !e.SMTP.TLS() && e.SMTP.Username != "" {
		return fmt.Errorf("email.smtp: use_tls: false can't be combined with a username (passwords are never sent unencrypted)")
	}
	return nil
}

// ValidateInbox validates inbox configuration (only called when inbox monitoring is used)
func (c *Config) ValidateInbox() error {
	return validateInboxConfig(c.Inbox)
}

func validateInboxConfig(inbox InboxConfig) error {
	if !inbox.Enabled {
		return fmt.Errorf("inbox: monitoring is not enabled in config")
	}
	if inbox.Email == "" {
		return fmt.Errorf("inbox: email address is required")
	}
	if inbox.Password == "" {
		if inbox.Auth == AuthOAuth2 {
			return fmt.Errorf("inbox: not signed in yet: run `delistd oauth login`")
		}
		return fmt.Errorf("inbox: password (app password) is required")
	}
	if inbox.Auth != "" && inbox.Auth != AuthOAuth2 {
		return fmt.Errorf("inbox: unknown auth %q (use oauth2 or leave empty)", inbox.Auth)
	}
	if inbox.Auth == AuthOAuth2 && OAuthClientID(inbox.ClientID) == "" {
		return fmt.Errorf("inbox: client_id is required with auth: oauth2")
	}
	if inbox.Server == "" {
		if inbox.Provider != "" {
			return fmt.Errorf("inbox: unknown provider %q - set server and port, or use one of: %s", inbox.Provider, strings.Join(providerIDsWithIMAP(), ", "))
		}
		return fmt.Errorf("inbox: server is required (or a provider preset: %s)", strings.Join(providerIDsWithIMAP(), ", "))
	}
	if inbox.Port == 0 {
		return fmt.Errorf("inbox: IMAP port is required")
	}
	return nil
}
