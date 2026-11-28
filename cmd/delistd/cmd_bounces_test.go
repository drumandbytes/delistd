package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/drumandbytes/delistd/internal/bounces"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/history"
)

// EverSent is cleanup-bounces' anti-spoofing guard: only brokers a profile
// really emailed, across every profile, even when one profile's history lookup fails.
func TestEverSentBrokerIDs(t *testing.T) {
	store, err := history.NewStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("history.NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Add(&history.Record{
		ProfileID: "maris", BrokerID: "spokeo", Email: "privacy@spokeo.com",
		Status: history.StatusSent, SentAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed maris/spokeo: %v", err)
	}
	if err := store.Add(&history.Record{
		ProfileID: "spouse", BrokerID: "whitepages", Email: "privacy@whitepages.com",
		Status: history.StatusSent, SentAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed spouse/whitepages: %v", err)
	}
	// A failed send doesn't count as "ever sent" - the bounce guard should
	// still refuse to act on a spoofed bounce naming this broker.
	if err := store.Add(&history.Record{
		ProfileID: "maris", BrokerID: "never-delivered", Email: "privacy@never-delivered.example",
		Status: history.StatusFailed, SentAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed maris/never-delivered: %v", err)
	}

	cfg := &config.Config{
		Profiles: []config.NamedProfile{
			{ID: "maris", Profile: config.Profile{FirstName: "Maris", LastName: "P", Email: "maris@example.com"}},
			{ID: "spouse", Profile: config.Profile{FirstName: "Spouse", LastName: "P", Email: "spouse@example.com"}},
		},
	}

	got := bounces.EverSent(cfg, store)

	want := map[string]bool{"spokeo": true, "whitepages": true}
	if len(got) != len(want) {
		t.Fatalf("EverSent() = %v, want %v", got, want)
	}
	for id := range want {
		if !got[id] {
			t.Errorf("expected %q in the ever-sent set, got %v", id, got)
		}
	}
	if got["never-delivered"] {
		t.Error("a failed send should not count as ever-sent")
	}
	if got["not-a-broker-anyone-emailed"] {
		t.Error("an unrelated broker id should not be in the set")
	}
}

// A legacy single-profile config is wrapped as the "default" profile; its history must still be found.
func TestEverSentBrokerIDsSingleProfileConfig(t *testing.T) {
	store, err := history.NewStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("history.NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Add(&history.Record{
		ProfileID: config.DefaultProfileID, BrokerID: "spokeo", Email: "privacy@spokeo.com",
		Status: history.StatusSent, SentAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed default/spokeo: %v", err)
	}

	cfg := &config.Config{
		Profiles: []config.NamedProfile{{ID: config.DefaultProfileID, Profile: config.Profile{FirstName: "Test", LastName: "User", Email: "test@example.com"}}},
	}

	got := bounces.EverSent(cfg, store)
	if !got["spokeo"] {
		t.Errorf("EverSent() = %v, want spokeo present via the default profile", got)
	}
}
