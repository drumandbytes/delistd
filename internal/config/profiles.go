package config

import (
	"fmt"
	"regexp"
	"strings"
)

// NamedProfile is a person's identity plus the stable ID used by --profile,
// history rows and the web UI's switcher.
type NamedProfile struct {
	// ID: short, lowercase, hyphenated. Stored verbatim in history.db, so
	// renaming it orphans the profile's existing history.
	ID      string `yaml:"id"`
	Profile `yaml:",inline"`
	// Mail overrides the shared email:/inbox: blocks for this profile only
	// (see EmailForProfile/InboxForProfile). nil = use the shared blocks.
	Mail *MailConfig `yaml:"mail,omitempty"`
}

// DefaultProfileID is the synthetic ID used for the legacy single Profile
// block when no profiles: list is configured, and is what pre-multi-profile
// history rows are attributed to after the profile_id migration.
const DefaultProfileID = "default"

// GetProfiles returns every profile; before setup that's one empty "default"
// profile, so callers always have someone to edit.
func (c *Config) GetProfiles() []NamedProfile {
	if len(c.Profiles) > 0 {
		return c.Profiles
	}
	return []NamedProfile{{ID: DefaultProfileID}}
}

// PrimaryProfile is the "default" profile if there is one, else the first:
// the one `delistd init` edits and fresh-profile forms prefill from.
func (c *Config) PrimaryProfile() NamedProfile {
	profiles := c.GetProfiles()
	for _, p := range profiles {
		if strings.EqualFold(p.ID, DefaultProfileID) {
			return p
		}
	}
	return profiles[0]
}

// HasProfile reports whether setup has stored anyone's details yet.
func (c *Config) HasProfile() bool {
	for _, p := range c.GetProfiles() {
		if p.FirstName != "" {
			return true
		}
	}
	return false
}

// GetProfile resolves a profile by ID. Empty id works only when exactly one
// profile exists; otherwise it errors with the available IDs.
func (c *Config) GetProfile(id string) (NamedProfile, error) {
	profiles := c.GetProfiles()
	if id == "" {
		if len(profiles) == 1 {
			return profiles[0], nil
		}
		return NamedProfile{}, fmt.Errorf("multiple profiles configured (%s) - specify one with --profile", strings.Join(profileIDs(profiles), ", "))
	}
	for _, p := range profiles {
		if strings.EqualFold(p.ID, id) {
			return p, nil
		}
	}
	return NamedProfile{}, fmt.Errorf("no profile %q configured (available: %s)", id, strings.Join(profileIDs(profiles), ", "))
}

func profileIDs(profiles []NamedProfile) []string {
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	return ids
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyID lowercases and hyphenates s, keeping only [a-z0-9]. Every profile
// ID must go through it: net/http silently drops non-ASCII bytes from
// Set-Cookie, so an ID with diacritics breaks the web UI's profile cookie.
func SlugifyID(s string) string {
	base := nonSlugChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "profile"
	}
	return base
}

// SlugifyProfileID derives an ID from first/last name (see SlugifyID),
// appending -2, -3, ... when taken.
func SlugifyProfileID(firstName, lastName string, existing []NamedProfile) string {
	base := SlugifyID(firstName + "-" + lastName)

	taken := make(map[string]bool, len(existing))
	for _, p := range existing {
		taken[strings.ToLower(p.ID)] = true
	}

	id := base
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

type Profile struct {
	FirstName string `yaml:"first_name"`
	// MiddleName: some brokers (background-check, financial-b2b) match on
	// full legal name.
	MiddleName string `yaml:"middle_name,omitempty"`
	LastName   string `yaml:"last_name"`
	Email      string `yaml:"email"`
	// AdditionalEmails: older addresses a broker may have indexed you under.
	AdditionalEmails []string `yaml:"additional_emails,omitempty"`
	// NameVariants covers other spellings brokers may have indexed you under -
	// e.g. a diacritic-free version of your name ("Maris" for "Māris"), a
	// maiden name, or a nickname you've used to sign up for things.
	NameVariants []string `yaml:"name_variants,omitempty"`
	// PreviousAddresses: partial ones still help, since matching keys off
	// street/city/postal code.
	PreviousAddresses []string `yaml:"previous_addresses,omitempty"`
	Address           string   `yaml:"address,omitempty"`
	City              string   `yaml:"city,omitempty"`
	State             string   `yaml:"state,omitempty"`
	ZipCode           string   `yaml:"zip_code,omitempty"`
	Country           string   `yaml:"country,omitempty"`
	Phone             string   `yaml:"phone,omitempty"`
	// AdditionalPhones covers other numbers you've used to sign up for
	// things (an old number, a work line) that a broker might have on file
	// instead of your current one.
	AdditionalPhones []string `yaml:"additional_phones,omitempty"`
	DateOfBirth      string   `yaml:"date_of_birth,omitempty"`
}

func (p Profile) FullName() string {
	if p.MiddleName == "" {
		return p.FirstName + " " + p.LastName
	}
	return p.FirstName + " " + p.MiddleName + " " + p.LastName
}
