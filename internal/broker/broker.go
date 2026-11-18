// Package broker holds the data-broker list: the entry type, loading and
// saving it, validating it, and picking the brokers a request should go to.
package broker

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/drumandbytes/delistd/internal/fsutil"
)

// Broker is one entry of brokers.yaml.
type Broker struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Email     string `yaml:"email"`
	Website   string `yaml:"website,omitempty"`
	OptOutURL string `yaml:"opt_out_url,omitempty"`
	Region    string `yaml:"region"`             // us, eu or global
	Category  string `yaml:"category,omitempty"` // people-search, marketing, background-check, ...
	Notes     string `yaml:"notes,omitempty"`
	// Countries limits the broker to people connected to these countries (ISO
	// 3166-1 alpha-2, e.g. ["DE"]): it holds one market's data and asks not to
	// be sent requests from elsewhere. Empty means everyone.
	Countries []string `yaml:"countries,omitempty"`

	// Optional research shown on the site's guide page. Only what the broker's
	// own pages state; Sources are those pages and Researched the day they were read.
	Parent       string   `yaml:"parent,omitempty"`        // operating company or owner
	HQ           string   `yaml:"hq,omitempty"`            // country or city of the operator
	Holds        string   `yaml:"holds,omitempty"`         // data categories they say they hold
	Verification string   `yaml:"verification,omitempty"`  // what they ask for before acting
	ResponseTime string   `yaml:"response_time,omitempty"` // what their policy says
	Sources      []string `yaml:"sources,omitempty"`
	Researched   string   `yaml:"researched,omitempty"` // YYYY-MM-DD
	Guide        bool     `yaml:"guide,omitempty"`      // give a long-tail broker its own guide page
}

// BrokerDatabase is the top level of a broker YAML document.
type BrokerDatabase struct {
	Brokers []Broker `yaml:"brokers"`
}

// Domain is the website's lower-case host without a leading "www.", or "" when
// there is no usable website.
func (b Broker) Domain() string {
	u, err := parseURL(b.Website)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// Domains is the website host with and without "www." (nil without a website),
// for matching links against the broker's own site.
func (b Broker) Domains() []string {
	if d := b.Domain(); d != "" {
		return []string{d, "www." + d}
	}
	return nil
}

// find returns the first broker satisfying match, or nil.
func (db *BrokerDatabase) find(match func(Broker) bool) *Broker {
	if i := slices.IndexFunc(db.Brokers, match); i >= 0 {
		return &db.Brokers[i]
	}
	return nil
}

func hasID(id string) func(Broker) bool {
	return func(b Broker) bool { return strings.EqualFold(b.ID, id) }
}

func hasEmail(email string) func(Broker) bool {
	return func(b Broker) bool { return strings.EqualFold(b.Email, email) }
}

func hasName(name string) func(Broker) bool {
	name = strings.TrimSpace(name)
	return func(b Broker) bool { return strings.EqualFold(strings.TrimSpace(b.Name), name) }
}

func (db *BrokerDatabase) FindByID(id string) *Broker       { return db.find(hasID(id)) }
func (db *BrokerDatabase) FindByName(name string) *Broker   { return db.find(hasName(name)) }
func (db *BrokerDatabase) FindByEmail(email string) *Broker { return db.find(hasEmail(email)) }

// Add appends b, refusing an ID that is already taken.
func (db *BrokerDatabase) Add(b Broker) error {
	if db.FindByID(b.ID) != nil {
		return fmt.Errorf("broker with ID %q already exists", b.ID)
	}
	db.Brokers = append(db.Brokers, b)
	return nil
}

// MarkEmailUnreachable clears a broker's email instead of deleting the entry
// (a bounce usually means a changed address, not a gone company) and appends a
// dated note with the old address and the reason. It returns the broker, or nil
// when none has that email.
func (db *BrokerDatabase) MarkEmailUnreachable(email, reason string) *Broker {
	b := db.FindByEmail(email)
	if b == nil {
		return nil
	}
	note := fmt.Sprintf("Email %s became undeliverable (%s, %s) - cleared, needs a working address.",
		b.Email, reason, time.Now().Format("2006-01-02"))
	b.Email = ""
	b.Notes = strings.TrimSpace(b.Notes + " " + note)
	return b
}

// Save writes the list as YAML, atomically.
func (db *BrokerDatabase) Save(path string) error {
	out, err := yaml.Marshal(db)
	if err != nil {
		return fmt.Errorf("failed to serialize brokers: %w", err)
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}

// SaveWithBackup copies an existing file to path+".bak" before saving.
func (db *BrokerDatabase) SaveWithBackup(path string) error {
	switch prev, err := os.ReadFile(path); {
	case err == nil:
		if err := os.WriteFile(path+".bak", prev, 0o644); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("failed to read file for backup: %w", err)
	}
	return db.Save(path)
}
