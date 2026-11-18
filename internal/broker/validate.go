package broker

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// MinSaneBrokerCount is the size below which the main list is taken to be
// truncated or corrupt, not small. update-brokers checks it before replacing
// the local copy, and Validate uses it.
const MinSaneBrokerCount = 200

// MinVerifiedBrokerCount is the same floor for the much smaller, hand-built
// verified list (data/brokers-verified.yaml).
const MinVerifiedBrokerCount = 20

// KnownRegions are the region values the rest of the code branches on. A typo
// such as "usa" would silently drop a broker from every region-filtered send.
var KnownRegions = []string{"us", "eu", "global"}

var (
	countryCode = regexp.MustCompile(`^[A-Z]{2}$`)
	isoDate     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// plausibleEmail only catches obvious breakage (no @, no dot in the
	// domain, whitespace), not RFC compliance.
	plausibleEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

func parseURL(raw string) (*url.URL, error) { return url.Parse(strings.TrimSpace(raw)) }

// isValidURL accepts an empty string and any http or https URL.
func isValidURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https"))
}

// dropInvalidURLs blanks the broker's URL fields that are not http(s).
func dropInvalidURLs(b *Broker) {
	if !isValidURL(b.OptOutURL) {
		b.OptOutURL = ""
	}
	if !isValidURL(b.Website) {
		b.Website = ""
	}
	b.Sources = slices.DeleteFunc(b.Sources, func(u string) bool { return !isValidURL(u) })
}

// Validate checks raw broker YAML without changing it (at least minCount
// entries, an id and a name each, unique ids, known regions, plausible emails,
// http(s) URLs) and reports every problem in one error. The parsed list is
// returned either way.
func Validate(raw []byte, minCount int) (*BrokerDatabase, error) {
	var db BrokerDatabase
	if err := yaml.Unmarshal(raw, &db); err != nil {
		return nil, fmt.Errorf("failed to parse broker data: %w", err)
	}

	var problems []string
	if n := len(db.Brokers); n < minCount {
		problems = append(problems, fmt.Sprintf("only %d brokers (expected at least %d) - looks truncated", n, minCount))
	}
	firstAt := make(map[string]int, len(db.Brokers))
	for i, b := range db.Brokers {
		id := strings.TrimSpace(b.ID)
		label := fmt.Sprintf("entry %d", i)
		if id != "" {
			label += " (" + id + ")"
		}
		switch prev, seen := firstAt[strings.ToLower(id)]; {
		case id == "":
			problems = append(problems, label+": missing id")
		case seen:
			problems = append(problems, fmt.Sprintf("%s: duplicate id, also at entry %d", label, prev))
		default:
			firstAt[strings.ToLower(id)] = i
		}
		for _, p := range b.Problems() {
			problems = append(problems, label+": "+p)
		}
	}
	if len(problems) == 0 {
		return &db, nil
	}
	return &db, fmt.Errorf("broker database has %d problem(s):\n  - %s", len(problems), strings.Join(problems, "\n  - "))
}

// Problems lists what is wrong with one entry's fields. Id uniqueness needs the
// whole list, so Validate checks that itself.
func (b Broker) Problems() []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	if strings.TrimSpace(b.Name) == "" {
		add("missing name")
	}
	if r := strings.ToLower(strings.TrimSpace(b.Region)); b.Region != "" && !slices.Contains(KnownRegions, r) {
		add("unknown region %q", b.Region)
	}
	for _, c := range b.Countries {
		if !countryCode.MatchString(c) {
			add("countries entry is not an upper-case ISO 3166-1 alpha-2 code: %q", c)
		}
	}
	if b.Email != "" && !plausibleEmail.MatchString(strings.TrimSpace(b.Email)) {
		add("implausible email %q", b.Email)
	}
	if !isValidURL(b.OptOutURL) {
		add("opt_out_url is not a valid http(s) URL: %q", b.OptOutURL)
	}
	for _, u := range b.Sources {
		if !isValidURL(u) {
			add("sources entry is not a valid http(s) URL: %q", u)
		}
	}
	if b.Researched != "" && !isoDate.MatchString(b.Researched) {
		add("researched is not YYYY-MM-DD: %q", b.Researched)
	}
	if !isValidURL(b.Website) {
		add("website is not a valid http(s) URL: %q", b.Website)
	}
	return out
}
