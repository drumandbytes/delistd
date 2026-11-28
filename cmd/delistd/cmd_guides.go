package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	emailtmpl "github.com/drumandbytes/delistd/internal/template"
	"github.com/spf13/cobra"
)

// curatedCategories get a full "how to opt out of <broker>" page. Everything
// else (the marketing/adtech long tail) is listed in the directory only -
// hundreds of near-identical thin pages would hurt the site, not help.
var curatedCategories = map[string]bool{
	"people-search":    true,
	"background-check": true,
	"financial-b2b":    true,
	"requires-id":      true,
	"device-id-only":   true,
}

// hasGuide: curated categories always get a page, and a long-tail broker gets one once someone
// has researched it and set guide: true in brokers.yaml.
func hasGuide(b broker.Broker) bool {
	return b.Guide || curatedCategories[strings.ToLower(b.Category)]
}

func guidesCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use: "guides",
		// maintainer tool for the project website; not part of the stable CLI
		Hidden: true,
		Short:  "Write the broker data the documentation site builds its guide pages from",
		Long: `Write brokers.json (the whole list, for the directory page) and guides.json
(one entry per opt-out guide: the broker's facts, the editorial text and the
rendered request letters) into the site's data directory. Hugo turns guides.json
into the "how to opt out of <broker>" pages (site/content/brokers/_content.gotmpl).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuides(output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "site/data", "Hugo data directory")

	return cmd
}

// placeholderProfile renders the request templates with fill-in-the-blank
// values instead of a real person's details.
func placeholderProfile() config.Profile {
	return config.Profile{
		FirstName: "[your first name]",
		LastName:  "[your last name]",
		Email:     "[your email address]",
		Address:   "[your street address]",
		City:      "[your city]",
		State:     "[your state/province]",
		ZipCode:   "[your postal code]",
		Country:   "[your country]",
	}
}

var dateInText = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)

// guideEntry is one opt-out page's data. The site's content adapter lays it out.
type guideEntry struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Region        string         `json:"region"`
	Category      string         `json:"category"`
	CategoryTitle string         `json:"category_title"`
	CategoryLabel string         `json:"category_label,omitempty"`
	CategoryIntro string         `json:"category_intro,omitempty"`
	Email         string         `json:"email,omitempty"`
	OptOutURL     string         `json:"opt_out_url,omitempty"`
	LastChecked   string         `json:"last_checked,omitempty"`
	Noindex       bool           `json:"noindex,omitempty"`
	Notes         string         `json:"notes,omitempty"`
	Timeline      string         `json:"timeline"`
	Law           string         `json:"law"`
	Route         string         `json:"route"`
	NeedsID       bool           `json:"needs_id,omitempty"`
	Parent        string         `json:"parent,omitempty"`
	HQ            string         `json:"hq,omitempty"`
	Holds         string         `json:"holds,omitempty"`
	Verification  string         `json:"verification,omitempty"`
	ResponseTime  string         `json:"response_time,omitempty"`
	Researched    string         `json:"researched,omitempty"`
	Sources       []string       `json:"sources,omitempty"`
	Related       []relatedGuide `json:"related,omitempty"`
	GDPR          string         `json:"gdpr,omitempty"`
	CCPA          string         `json:"ccpa,omitempty"`
}

type relatedGuide struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// noindexedBrokers: reachable but kept out of search and the sitemap, as they
// offer nothing actionable. Curated by hand; a dead end is an editorial call.
var noindexedBrokers = map[string]bool{
	"publicrecordsnow": true, // no email or opt-out URL on file; the one we had bounced
	"regis24":          true, // DE/AT-only credit bureau; explicit no-further-action reply
}

// timelineText names the deadline a broker's reply is legally bound by.
// Region-gated: previously this sentence cited both GDPR's one month and the
// CCPA's 45 days on every page, including e.g. a German-only credit bureau
// with no CCPA exposure at all.
func timelineText(region string) string {
	switch region {
	case "eu":
		return "Under GDPR they must respond within one month."
	case "us":
		return "Under the CCPA, they must respond within 45 days."
	default: // "global" - could be either, depending on the requester
		return "Under GDPR they must respond within one month if you're in the EU/EEA/UK, or within 45 days under the CCPA if you're in the US."
	}
}

// categoryInfo explains, per curated category, what these brokers do and what removal looks
// like. Shared by every page in the category, so keep it general and true of all of them.
var categoryInfo = map[string]struct{ Label, Intro string }{
	"marketing":        {"Marketing-data broker", "Marketing-data brokers compile consumer or business contact data and audience segments for advertisers, usually without a public search site. Removal means an opt-out or deletion request to the company itself, and you may need to repeat it for each brand or affiliate it operates."},
	"people-search":    {"People-search site", "People-search sites assemble public records (addresses, phone numbers, ages, relatives) into searchable profiles. Most have a web form, and many ask you to confirm by email. Profiles often reappear after a few months as the underlying records are re-ingested, so check again every 60-90 days."},
	"background-check": {"Background-check company", "Background-check and screening companies sell reports built from court, address and employment records to employers, landlords and the public. Some are consumer reporting agencies, which gives you access and dispute rights as well as deletion, and the law can require them to keep some records. Ask for access first if you want to see what they hold."},
	"financial-b2b":    {"Credit bureau or business-data provider", "Credit bureaus and business-data providers hold financial and company-contact data. Credit reporting is regulated, so erasure is often limited to data held beyond what the law requires; access, correction and objection rights usually still apply. Expect a longer, more formal reply than from a people-search site."},
	"requires-id":      {"Broker that verifies identity first", "These brokers ask you to prove who you are before they act, usually with a copy of an ID or a verification form, so that nobody else can delete your record. Send only what their own page asks for, and cover or redact anything they do not need."},
	"device-id-only":   {"Mobile and device-data broker", "These companies hold mobile advertising IDs, location and app data rather than names. Their forms usually need your device's advertising ID, so reset or delete it in your phone's privacy settings as well, otherwise a new ID starts collecting again."},
}

// lawText names the law that gives the requester a deletion right, by broker region.
func lawText(region string) string {
	switch region {
	case "eu":
		return "EU / EEA (GDPR Article 17)"
	case "us":
		return "United States (CCPA / CPRA and other state laws)"
	default:
		return "Global (GDPR Article 17 for EU/EEA/UK residents, CCPA / CPRA for California)"
	}
}

func routeText(b broker.Broker) string {
	switch {
	case b.OptOutURL != "" && b.Email != "":
		return "Web form, with an email fallback"
	case b.OptOutURL != "":
		return "Web form"
	case b.Email != "":
		return "Email"
	default:
		return "No public channel on file"
	}
}

// relatedGuides returns up to n other guides from the same category: the ones that follow b in
// name order, wrapping around, so every guide in a category gets about the same number of
// incoming links. Noindexed brokers are not linked to.
func relatedGuides(b broker.Broker, curated []broker.Broker, n int) []relatedGuide {
	var same []broker.Broker
	idx := -1
	for _, o := range curated {
		if !strings.EqualFold(o.Category, b.Category) {
			continue
		}
		if o.ID == b.ID {
			idx = len(same)
		}
		same = append(same, o)
	}
	var out []relatedGuide
	for i := 1; i < len(same) && len(out) < n; i++ {
		o := same[(idx+i)%len(same)]
		if noindexedBrokers[o.ID] {
			continue
		}
		out = append(out, relatedGuide{ID: o.ID, Name: o.Name})
	}
	return out
}

// guideDescription is the meta description / JSON-LD description for a
// broker page - previously the site-wide default, verbatim, on every page.
func guideDescription(b broker.Broker, lastChecked string, hasLetter bool) string {
	var method string
	switch {
	case b.OptOutURL != "":
		method = "submit their opt-out form"
	case b.Email != "":
		method = "email a data-removal request"
	default:
		method = "contact them directly"
	}

	var window string
	switch b.Region {
	case "eu":
		window = "GDPR gives them one month to respond"
	case "us":
		window = "CCPA gives them 45 days to respond"
	default:
		window = "GDPR or CCPA applies depending on your location"
	}

	desc := fmt.Sprintf("How to opt out of %s: %s. %s.", b.Name, method, window)
	if hasLetter {
		desc += " Includes a ready-to-send request letter."
	}
	if lastChecked != "" {
		desc = fmt.Sprintf("%s Checked %s.", desc, lastChecked)
	}
	return truncateRunes(desc, 160)
}

// truncateRunes cuts by rune, not byte, count -- broker names carry accented
// characters (Buró, Crédito, Investigación), and byte-slicing a UTF-8 string
// can split one in half.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

func runGuides(dataDir string) error {
	db, err := broker.Load(brokerFile)
	if err != nil {
		return fmt.Errorf("failed to load brokers: %w", err)
	}
	engine, err := emailtmpl.NewEngine()
	if err != nil {
		return fmt.Errorf("failed to init template engine: %w", err)
	}
	pp := placeholderProfile()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	var curated []broker.Broker
	for _, b := range db.Brokers {
		if hasGuide(b) {
			curated = append(curated, b)
		}
	}
	slices.SortFunc(curated, func(a, b broker.Broker) int { return cmp.Compare(a.Name, b.Name) })

	guides := make([]guideEntry, 0, len(curated))
	for _, b := range curated {
		g := guideEntry{
			ID: b.ID, Name: b.Name, Title: "How to opt out of " + b.Name,
			Region: b.Region, Category: b.Category, CategoryTitle: titleCase(b.Category),
			Email: b.Email, OptOutURL: b.OptOutURL, Notes: b.Notes,
			Timeline: timelineText(b.Region), Law: lawText(b.Region), Route: routeText(b),
			Parent: b.Parent, HQ: b.HQ, Holds: b.Holds, Verification: b.Verification,
			ResponseTime: b.ResponseTime, Researched: b.Researched, Sources: b.Sources,
			Related: relatedGuides(b, curated, 5), Noindex: noindexedBrokers[b.ID],
		}
		if m := dateInText.FindStringSubmatch(b.Notes); m != nil {
			g.LastChecked = m[1]
		}
		cat := strings.ToLower(b.Category)
		g.CategoryLabel, g.CategoryIntro, g.NeedsID = categoryInfo[cat].Label, categoryInfo[cat].Intro, cat == "requires-id"
		if gdpr, err := engine.Render("gdpr", pp, b); err == nil {
			g.GDPR = gdpr.Body
		}
		if ccpa, err := engine.Render("ccpa", pp, b); err == nil {
			g.CCPA = ccpa.Body
		}
		g.Description = guideDescription(b, g.LastChecked, g.GDPR != "" || g.CCPA != "")
		guides = append(guides, g)
	}

	type dirEntry struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Region   string `json:"region"`
		Category string `json:"category"`
		Email    string `json:"email,omitempty"`
		OptOut   string `json:"opt_out_url,omitempty"`
		Curated  bool   `json:"curated"`
		Notes    string `json:"notes,omitempty"`
	}
	dir := make([]dirEntry, 0, len(db.Brokers))
	for _, b := range db.Brokers {
		dir = append(dir, dirEntry{
			ID: b.ID, Name: b.Name, Region: b.Region, Category: b.Category,
			Email: b.Email, OptOut: b.OptOutURL, Curated: hasGuide(b), Notes: b.Notes,
		})
	}
	slices.SortFunc(dir, func(a, b dirEntry) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })

	// Hugo data files must be objects, not top-level arrays.
	for name, v := range map[string]any{"guides.json": map[string]any{"guides": guides}, "brokers.json": map[string]any{"brokers": dir}} {
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dataDir, name), out, 0o644); err != nil {
			return err
		}
	}

	fmt.Println("📚 Guide data written")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   %d guide(s) and a directory of %d brokers in %s\n", len(guides), len(dir), dataDir)
	return nil
}

// titleCase turns "people-search" into "People Search".
func titleCase(s string) string {
	words := strings.Fields(strings.ReplaceAll(s, "-", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
