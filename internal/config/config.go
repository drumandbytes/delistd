package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/drumandbytes/delistd/internal/datadir"
	"github.com/drumandbytes/delistd/internal/fsutil"
)

const (
	// DefaultRateLimitMs is the pause between emails.
	DefaultRateLimitMs = 2000
	// DefaultDailySendLimit stays under Gmail's ~500/day for a regular account,
	// leaving room for the other mail sent that day.
	DefaultDailySendLimit = 450

	defaultBrowserTimeoutSec = 30
)

// Config is the contents of config.yaml.
type Config struct {
	// Profiles are the people requests are sent for. They share the brokers,
	// the mail account and the options; history is kept per profile.
	Profiles []NamedProfile `yaml:"profiles,omitempty"`
	// PassphraseProtected means the mail passwords are encrypted with a
	// passphrase on top of the machine binding (secret.go). Set by `delistd
	// passphrase set`.
	PassphraseProtected bool        `yaml:"passphrase_protected,omitempty"`
	Email               EmailConfig `yaml:"email"`              // shared SMTP account
	Options             Options     `yaml:"options"`            // sending and selection
	Inbox               InboxConfig `yaml:"inbox,omitempty"`    // shared IMAP account
	Pipeline            Pipeline    `yaml:"pipeline,omitempty"` // browser automation
	Schedule            Schedule    `yaml:"schedule,omitempty"` // in-app scheduler
}

// DefaultConfigPath is ~/.delistd/config.yaml (or the legacy directory), or a
// bare config.yaml when there is no home directory.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(datadir.Of(home), "config.yaml")
}

// warnIfReadable prints a warning when other users can read the file.
func warnIfReadable(path string) {
	if err := checkFilePermissions(path); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}
}

func checkFilePermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("config file %s has insecure permissions %04o; should be 0600", path, perm)
	}
	return nil
}

// Load reads and decrypts the config at path, fills the defaults and upgrades
// a 1.x file in place (migrate.go).
func Load(path string) (*Config, error) {
	warnIfReadable(path)

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.applyDefaults()
	plaintext := cfg.decryptPasswords()
	migrateFrom1x(path, raw, &cfg, plaintext)
	cfg.applyInboxDefaults()
	return &cfg, nil
}

// applyDefaults fills only genuinely missing fields; an explicit value wins.
// The template default is gdpr because the tool is aimed at EU use
// (EU-NOTES.md). BrowserHeadless stays nil when unset: Pipeline.Headless
// supplies true without overwriting an explicit false.
func (c *Config) applyDefaults() {
	if c.Options.Template == "" {
		c.Options.Template = "gdpr"
	}
	if c.Options.RateLimitMs == 0 {
		c.Options.RateLimitMs = DefaultRateLimitMs
	}
	if c.Options.DailySendLimit == 0 {
		c.Options.DailySendLimit = DefaultDailySendLimit
	}
	if c.Pipeline.BrowserTimeoutSec == 0 {
		c.Pipeline.BrowserTimeoutSec = defaultBrowserTimeoutSec
	}
}

// decryptPasswords turns each stored password into plaintext in memory and
// reports whether any was stored unencrypted (a legacy file, to be re-saved
// encrypted). Passphrase-protected values stay as they are until Reveal.
func (c *Config) decryptPasswords() (foundPlaintext bool) {
	c.eachSecret(func(p *string) {
		switch {
		case *p != "" && !isEncrypted(*p):
			foundPlaintext = true
		case strings.HasPrefix(*p, secretPrefixV2):
		default:
			plain, err := decryptSecret(*p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: %v; re-enter it with `delistd init` or in Settings\n", err)
				plain = ""
			}
			*p = plain
		}
	})
	return foundPlaintext
}

// applyInboxDefaults fills the server settings of the shared inbox and of each
// profile's own one.
func (c *Config) applyInboxDefaults() {
	ApplyInboxDefaults(&c.Inbox)
	for _, p := range c.Profiles {
		if p.Mail != nil && p.Mail.Inbox != nil {
			ApplyInboxDefaults(p.Mail.Inbox)
		}
	}
}

// Save encrypts the passwords and writes the config atomically with 0600.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	out := cfg.Clone() // cfg may be the live config others are reading
	protect := func(p *string) error {
		if out.PassphraseProtected && *p != "" && !isEncrypted(*p) {
			pass, err := passphrase()
			if err != nil {
				return err
			}
			*p = encryptV2(*p, pass)
			return nil
		}
		*p = encryptSecret(*p)
		return nil
	}
	if err := out.secretFields(protect); err != nil {
		return fmt.Errorf("failed to protect passwords: %w", err)
	}
	doc, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}
	return fsutil.WriteFileAtomic(path, doc, 0o600)
}

// Validate checks every profile and, unless sending is manual, the shared
// email account.
func (c *Config) Validate() error {
	manual := c.IsManualSend()
	seen := map[string]bool{}
	for _, np := range c.GetProfiles() {
		if np.ID == "" {
			return fmt.Errorf("profiles: every profile needs an id")
		}
		key := strings.ToLower(np.ID)
		if seen[key] {
			return fmt.Errorf("profiles: duplicate profile id %q", np.ID)
		}
		seen[key] = true

		switch {
		case np.FirstName == "" || np.LastName == "":
			return fmt.Errorf("profile %q: first_name and last_name are required", np.ID)
		case np.Email == "":
			return fmt.Errorf("profile %q: email is required", np.ID)
		}
		if !manual && np.Mail != nil && np.Mail.Email != nil {
			if err := validateEmailConfig(*np.Mail.Email); err != nil {
				return fmt.Errorf("profile %q: mail.email: %w", np.ID, err)
			}
		}
	}
	if manual {
		return nil // nothing is sent, so no mail account is needed
	}
	return validateEmailConfig(c.Email)
}
