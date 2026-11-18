package history

import (
	"testing"
	"time"
)

// A store whose database is gone must report it from every migration step
// instead of carrying on with half a schema.
func TestMigrationsFailOnAClosedDatabase(t *testing.T) {
	s := newTestStore(t)
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	steps := map[string]func() error{
		"migrate":           s.migrate,
		"migrateTimestamps": s.migrateTimestamps,
		"migrateApprovals":  s.migrateApprovals,
		"migrateAutomation": s.migrateAutomation,
		"MigrateNoData": func() error {
			return s.MigrateNoData(func(BrokerResponse) (bool, float64, bool) { return true, 1, true })
		},
		"ResolveNoData": func() error { return s.ResolveNoData("p", "b", true) },
	}
	for name, step := range steps {
		if err := step(); err == nil {
			t.Errorf("%s succeeded on a closed database", name)
		}
	}
}

// Every query reports a database that is gone instead of returning empty
// results that look like "no history".
func TestQueriesFailOnAClosedDatabase(t *testing.T) {
	s := newTestStore(t)
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	calls := map[string]func() error{
		"Add":                     func() error { return s.Add(&Record{ProfileID: "p", BrokerID: "b", Status: StatusSent, SentAt: now}) },
		"GetRecentRequests":       func() error { _, err := s.GetRecentRequests("p", 5); return err },
		"GetAllRequests":          func() error { _, err := s.GetAllRequests("p"); return err },
		"LatestRequests":          func() error { _, err := s.LatestRequests("p"); return err },
		"GetStats":                func() error { _, _, _, err := s.GetStats("p"); return err },
		"GetMonthlyStats":         func() error { _, _, err := s.GetMonthlyStats("p"); return err },
		"CountSentSince":          func() error { _, err := s.CountSentSince("p", now); return err },
		"LastSuccessfulSendTimes": func() error { _, err := s.LastSuccessfulSendTimes("p"); return err },
		"MarkFailed":              func() error { _, err := s.MarkFailed("p", "b", "n"); return err },
		"ResolveProfileForBroker": func() error { _, err := s.ResolveProfileForBroker("b"); return err },
		"GetAllBrokerStatuses":    func() error { _, err := s.GetAllBrokerStatuses("p"); return err },
		"GetBrokerStatus":         func() error { _, err := s.GetBrokerStatus("p", "b"); return err },
		"DeleteByStatus":          func() error { _, err := s.DeleteByStatus("p", StatusFailed); return err },
		"DeleteAllHistory":        func() error { _, err := s.DeleteAllHistory("p"); return err },
		"GetPipelineStats":        func() error { _, err := s.GetPipelineStats("p"); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s succeeded on a closed database", name)
		}
	}
}
