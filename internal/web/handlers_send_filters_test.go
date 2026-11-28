package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drumandbytes/delistd/internal/broker"
)

func TestGetBrokersWithStatusRecipientFilters(t *testing.T) {
	s := newTestServer(t, testConfig())
	s.brokers().Brokers = []broker.Broker{
		{ID: "alpha", Name: "Alpha", Email: "privacy@alpha.example", Region: "us", Category: "marketing"},
		{ID: "beta", Name: "Beta", Email: "privacy@beta.example", Region: "eu", Category: "people-search"},
		{ID: "noemail", Name: "No Email", Region: "eu", Category: "people-search"},
	}

	got := s.listBrokers("default", brokerQuery{Search: "alpha", Category: "marketing", Region: "us", Status: "all", Only: []string{"alpha", "beta"}, Skip: []string{"beta"}})
	if len(got) != 1 || got[0].ID != "alpha" {
		t.Fatalf("filtered brokers = %+v, want alpha", got)
	}
}

func TestHandleAPISendAllRejectsUnknownBrokerID(t *testing.T) {
	cfg := testConfig()
	cfg.Email.SMTP.Host = "smtp.example.com"
	s := newTestServer(t, cfg)
	s.brokers().Brokers = []broker.Broker{{ID: "alpha", Name: "Alpha", Email: "privacy@alpha.example", Region: "us"}}

	form := url.Values{"broker_ids": {"missing"}}
	req := httptest.NewRequest(http.MethodPost, "/api/send-all", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPISendAll(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Unknown broker IDs: missing") {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
}
