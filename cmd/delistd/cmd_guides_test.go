package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drumandbytes/delistd/internal/broker"
)

type guidesFile struct {
	Guides []guideEntry `json:"guides"`
}

func readGuides(t *testing.T, dataDir string) map[string]guideEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, "guides.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f guidesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	m := map[string]guideEntry{}
	for _, g := range f.Guides {
		m[g.ID] = g
	}
	return m
}

func useBrokers(t *testing.T, yaml string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "brokers.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func(b string) func() { return func() { brokerFile = b } }(brokerFile))
	brokerFile = path
}

func TestRunGuidesData(t *testing.T) {
	useBrokers(t, `brokers:
  - id: acme
    name: Acme People Finder
    email: privacy@acme.example
    opt_out_url: https://acme.example/opt-out
    region: us
    category: people-search
    notes: Replied 2026-08-01, removed within a week.
  - id: globex-ads
    name: Globex Ads
    email: dpo@globex.example
    region: eu
    category: marketing
  - id: nocontact
    name: No Contact Bureau
    region: us
    category: background-check
`)
	out := t.TempDir()
	if err := runGuides(out); err != nil {
		t.Fatalf("runGuides: %v", err)
	}

	// Curated: people-search + background-check get guides; marketing does not.
	guides := readGuides(t, out)
	if _, ok := guides["acme"]; !ok {
		t.Error("acme has no guide")
	}
	if _, ok := guides["nocontact"]; !ok {
		t.Error("nocontact has no guide")
	}
	if _, ok := guides["globex-ads"]; ok {
		t.Error("marketing broker should not get a guide")
	}

	acme := guides["acme"]
	if acme.Title != "How to opt out of Acme People Finder" || acme.LastChecked != "2026-08-01" ||
		acme.CategoryTitle != "People Search" || acme.CategoryLabel != "People-search site" ||
		acme.Route != "Web form, with an email fallback" || !strings.HasPrefix(acme.Law, "United States (CCPA") {
		t.Errorf("acme entry: %+v", acme)
	}
	if !strings.Contains(acme.Description, "Includes a ready-to-send request letter.") {
		t.Errorf("description does not mention the letters: %q", acme.Description)
	}
	for name, want := range map[string]string{"GDPR": "Article 17", "CCPA": "1798.105", "placeholder": "[your first name]"} {
		if !strings.Contains(acme.GDPR+acme.CCPA, want) {
			t.Errorf("letters miss the %s text %q", name, want)
		}
	}

	// The directory lists every broker, with the curated flag set.
	raw, err := os.ReadFile(filepath.Join(out, "brokers.json"))
	if err != nil {
		t.Fatal(err)
	}
	var dir struct {
		Brokers []struct {
			ID      string `json:"id"`
			Curated bool   `json:"curated"`
		} `json:"brokers"`
	}
	if err := json.Unmarshal(raw, &dir); err != nil {
		t.Fatal(err)
	}
	if len(dir.Brokers) != 3 {
		t.Fatalf("directory has %d entries, want 3", len(dir.Brokers))
	}
	byID := map[string]bool{}
	for _, e := range dir.Brokers {
		byID[e.ID] = e.Curated
	}
	if !byID["acme"] || byID["globex-ads"] {
		t.Errorf("curated flags wrong: %+v", byID)
	}
}

func TestRelatedGuides(t *testing.T) {
	mk := func(id, cat string) broker.Broker {
		return broker.Broker{ID: id, Name: strings.ToUpper(id), Category: cat}
	}
	curated := []broker.Broker{mk("a", "people-search"), mk("b", "people-search"), mk("c", "background-check"), mk("d", "people-search"), mk("e", "people-search")}

	// Next ones in name order within the category, wrapping, never itself or another category.
	got := relatedGuides(curated[3], curated, 5)
	if len(got) != 3 || got[0].ID != "e" || got[1].ID != "a" || got[2].ID != "b" {
		t.Errorf("related for d = %+v, want e, a, b", got)
	}
	if got := relatedGuides(curated[2], curated, 5); len(got) != 0 {
		t.Errorf("a category of one should have no related guides, got %+v", got)
	}
	if got := relatedGuides(curated[0], curated, 2); len(got) != 2 {
		t.Errorf("limit not applied: %+v", got)
	}
}

func TestGuidesResearchedFields(t *testing.T) {
	useBrokers(t, `brokers:
  - id: adco
    name: Ad Co
    email: privacy@adco.example
    region: us
    category: marketing
    guide: true
    parent: Ad Holdings Inc.
    hq: Austin, USA
    holds: email addresses and device IDs
    verification: none stated
    response_time: 45 days
    sources: ["https://adco.example/privacy", "not a url"]
    researched: "2026-10-25"
  - id: plain
    name: Plain Co
    region: us
    category: marketing
`)
	out := t.TempDir()
	if err := runGuides(out); err != nil {
		t.Fatalf("runGuides: %v", err)
	}
	guides := readGuides(t, out)
	if _, ok := guides["plain"]; ok {
		t.Error("a long-tail broker without guide: true must not get a guide")
	}
	g, ok := guides["adco"]
	if !ok {
		t.Fatal("guide: true broker has no guide")
	}
	if g.Parent != "Ad Holdings Inc." || g.HQ != "Austin, USA" || g.Holds != "email addresses and device IDs" ||
		g.Verification != "none stated" || g.ResponseTime != "45 days" || g.Researched != "2026-10-25" ||
		g.CategoryLabel != "Marketing-data broker" {
		t.Errorf("researched fields not carried: %+v", g)
	}
	// An invalid source URL is dropped when the list is loaded.
	if len(g.Sources) != 1 || g.Sources[0] != "https://adco.example/privacy" {
		t.Errorf("sources = %v", g.Sources)
	}
}
