package config

import "strings"

// Options are the sending and broker-selection settings.
type Options struct {
	Template string `yaml:"template"` // gdpr, ccpa, generic or custom
	// CustomTemplate is the body of the user-written "custom" template (Go
	// text/template with the built-in fields); an optional first line
	// "Subject: ..." sets the subject. Used when Template is "custom".
	CustomTemplate string `yaml:"custom_template,omitempty"`
	// SendMode is "" or "smtp" (Delistd sends over the SMTP account) or "manual":
	// Delistd renders the emails for you to send and record with `mark-sent`,
	// and needs no email: block, for people who won't share mailbox credentials.
	SendMode    string `yaml:"send_mode,omitempty"`
	DryRun      bool   `yaml:"dry_run,omitempty"` // render only, send nothing
	RateLimitMs int    `yaml:"rate_limit_ms"`     // pause between emails
	// DailySendLimit caps sends per rolling 24h to stay under the provider's
	// limit; 0 means DefaultDailySendLimit. --ignore-daily-limit bypasses it.
	DailySendLimit int `yaml:"daily_send_limit,omitempty"`
	// BrokerList is "" or "full" (the default) or "verified"
	// (data/brokers-verified.yaml). `send --list` overrides it; BrokerFile and
	// --brokers win over it.
	BrokerList string `yaml:"broker_list,omitempty"`
	// BrokerFile points the commands that send at your own list, like the
	// global --brokers flag (which still wins) and ahead of BrokerList.
	BrokerFile      string   `yaml:"broker_file,omitempty"`
	Regions         []string `yaml:"regions,omitempty"`          // restrict to these regions
	ExcludedBrokers []string `yaml:"excluded_brokers,omitempty"` // by ID or name
	// ExcludedCategories skips brokers by category (case-insensitive), e.g.
	// requires-id for those that demand an ID document.
	ExcludedCategories []string `yaml:"excluded_categories,omitempty"`
}

// IsManualSend reports whether Delistd only renders the emails and never
// transmits them.
func (c *Config) IsManualSend() bool {
	return strings.EqualFold(c.Options.SendMode, "manual")
}

// Pipeline holds the browser-automation settings.
type Pipeline struct {
	// BrowserHeadless is a pointer so an explicit false survives loading;
	// read it through Headless().
	BrowserHeadless   *bool `yaml:"browser_headless,omitempty"`
	BrowserTimeoutSec int   `yaml:"browser_timeout_sec"` // per browser operation
}

// Headless is the effective setting: true unless the config says false.
func (p Pipeline) Headless() bool { return p.BrowserHeadless == nil || *p.BrowserHeadless }

// Schedule is the in-app fallback for automated cycles when no OS job is installed.
type Schedule struct {
	// Enabled makes a running `delistd serve` run a cycle every 6 hours. It is
	// ignored while the OS job is installed.
	Enabled bool `yaml:"enabled,omitempty"`
}
