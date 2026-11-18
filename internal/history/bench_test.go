package history

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A year of use: ~720 brokers re-sent a few times, replies with full bodies,
// two profiles. The dashboard and pipeline pages run these on every load.
func benchStore(b *testing.B) *Store {
	b.Helper()
	s, err := NewStore(filepath.Join(b.TempDir(), "h.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = s.Close() })
	body := strings.Repeat("Thank you for contacting us. We have received your request and will process it shortly. ", 40)
	now := time.Now()
	for _, p := range []string{"default", "partner"} {
		for i := 0; i < 720; i++ {
			for r := 0; r < 3; r++ {
				rec := &Record{ProfileID: p, BrokerID: fmt.Sprintf("broker-%d", i), BrokerName: fmt.Sprintf("Broker %d", i), Email: "x@y.z",
					Template: "gdpr", Status: StatusSent, SentAt: now.Add(-time.Duration(r*30+i%20) * 24 * time.Hour)}
				if err := s.Add(rec); err != nil {
					b.Fatal(err)
				}
			}
			if i%2 == 0 {
				if err := s.AddBrokerResponse(&BrokerResponse{ProfileID: p, BrokerID: fmt.Sprintf("broker-%d", i), BrokerName: fmt.Sprintf("Broker %d", i),
					ResponseType: "pending", EmailFrom: "privacy@x.z", EmailSubject: fmt.Sprintf("Ticket #%d", i), EmailBody: body, ReceivedAt: now, ProcessedAt: now}); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	return s
}

func BenchmarkStore(b *testing.B) {
	s := benchStore(b)
	run := func(name string, f func() error) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := f(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	run("GetStats", func() error { _, _, _, err := s.GetStats("default"); return err })
	run("GetAllBrokerStatuses", func() error { _, err := s.GetAllBrokerStatuses("default"); return err })
	run("LastSuccessfulSendTimes", func() error { _, err := s.LastSuccessfulSendTimes("default"); return err })
	run("LatestRequests", func() error { _, err := s.LatestRequests("default"); return err })
	run("GetPipelineStats", func() error { _, err := s.GetPipelineStats("default"); return err })
	run("GetResponseStats", func() error { _, err := s.GetResponseStats("default"); return err })
	run("GetRecentRequests", func() error { _, err := s.GetRecentRequests("default", 5); return err })
	run("GetBrokerResponses", func() error { _, err := s.GetBrokerResponses("default", "", false, 50); return err })
	run("GetActionItems", func() error { _, err := s.GetActionItems("default"); return err })
	run("QueryBrokerResponses", func() error {
		_, _, err := s.QueryBrokerResponses("default", ResponseQuery{Search: "ticket"})
		return err
	})
	run("CountSentSince", func() error { _, err := s.CountSentSince("default", time.Now().Add(-24*time.Hour)); return err })
}
