package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/history"
)

func TestFilterBrokersByStatus(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	brokers := []broker.Broker{{ID: "never"}, {ID: "failed"}, {ID: "recent"}, {ID: "old"}}
	statuses := map[string]history.BrokerStatus{
		"failed": {Status: history.StatusFailed, LastSent: now.Add(-time.Hour)},
		"recent": {Status: history.StatusSent, LastSent: now.Add(-24 * time.Hour)},
		"old":    {Status: history.StatusSent, LastSent: now.Add(-26 * 24 * time.Hour)},
	}

	cases := []struct {
		status string
		want   []string
	}{
		{"eligible", []string{"never", "failed", "old"}},
		{"never", []string{"never"}},
		{"failed", []string{"failed"}},
		{"all", []string{"never", "failed", "recent", "old"}},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			got := filterBrokersByStatus(brokers, statuses, tc.status, now)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i].ID != tc.want[i] {
					t.Fatalf("got %+v, want %v", got, tc.want)
				}
			}
		})
	}
}

// Email-less brokers sort first (never sent) but must not use up budget.
func TestCapSendsIgnoresEmaillessBrokers(t *testing.T) {
	brokers := []broker.Broker{{ID: "n1"}, {ID: "a", Email: "a@x"}, {ID: "n2"}, {ID: "b", Email: "b@x"}, {ID: "c", Email: "c@x"}}
	got := capSends(brokers, 2)
	var ids []string
	for _, b := range got {
		ids = append(ids, b.ID)
	}
	if strings.Join(ids, ",") != "n1,a,n2,b" {
		t.Errorf("capSends = %v, want n1,a,n2,b", ids)
	}
	if n := countWithEmail(brokers); n != 3 {
		t.Errorf("countWithEmail = %d", n)
	}
	if len(capSends(brokers, 5)) != 5 {
		t.Error("budget above the list size trimmed it")
	}
}

// A history database that fails must stop a send, not let it run unchecked
// against the daily cap or the resend cooldown.
func TestSendStepsStopWhenHistoryFails(t *testing.T) {
	store, err := history.NewStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	profile := config.NamedProfile{ID: "default"}
	list := []broker.Broker{{ID: "a", Email: "a@example.com"}}

	if _, err := eligibleBrokers(store, profile, list); err == nil {
		t.Error("eligibleBrokers ignored a failing history store")
	}
	cfg := &config.Config{Options: config.Options{DailySendLimit: 10}}
	if _, err := applyDailyCap(store, cfg, profile, list); err == nil {
		t.Error("applyDailyCap ignored a failing history store")
	}
}

func TestEligibleBrokersRejectsAnUnknownStatus(t *testing.T) {
	store, err := history.NewStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	defer func(s string) { statusFilter = s }(statusFilter)
	statusFilter = "sent"
	if _, err := eligibleBrokers(store, config.NamedProfile{ID: "default"}, []broker.Broker{{ID: "a"}}); err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("err = %v", err)
	}
}
