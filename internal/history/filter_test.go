package history

import (
	"testing"
	"time"
)

func TestMatchesFilter(t *testing.T) {
	now := time.Now()
	sentRecent := &BrokerStatus{Status: StatusSent, LastSent: now.Add(-time.Hour)}
	sentOld := &BrokerStatus{Status: StatusSent, LastSent: now.Add(-ResendCooldown - time.Hour)}
	failed := &BrokerStatus{Status: StatusFailed, LastSent: now}
	cases := []struct {
		filter string
		st     *BrokerStatus
		want   bool
	}{
		{"all", nil, true}, {"all", sentRecent, true}, {"bogus", failed, true},
		{"never", nil, true}, {"never", sentRecent, false}, {"pending", nil, true}, {"pending", failed, false},
		{"sent", sentRecent, true}, {"sent", failed, false}, {"sent", nil, false},
		{"failed", failed, true}, {"failed", sentRecent, false}, {"failed", nil, false},
		{"eligible", nil, true}, {"eligible", failed, true}, {"eligible", sentOld, true}, {"eligible", sentRecent, false},
	}
	for _, c := range cases {
		if got := MatchesFilter(c.filter, c.st, now); got != c.want {
			t.Errorf("MatchesFilter(%q, %v) = %v, want %v", c.filter, c.st, got, c.want)
		}
	}
}

func TestIsSendFilter(t *testing.T) {
	for _, ok := range []string{"eligible", "never", "failed", "all"} {
		if !IsSendFilter(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "sent", "pending", "ALL"} {
		if IsSendFilter(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
