package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drumandbytes/delistd/internal/broker"
)

func TestBrokerListRendersOnePageAtATime(t *testing.T) {
	s := smokeServer(t)
	s.brokers().Brokers = nil
	for i := 0; i < 120; i++ {
		s.brokers().Brokers = append(s.brokers().Brokers, broker.Broker{
			ID: fmt.Sprintf("b%03d", i), Name: fmt.Sprintf("Broker %03d", i), Email: fmt.Sprintf("p%d@b.example", i), Region: "us", Category: "marketing"})
	}
	get := func(path string) string {
		rec := httptest.NewRecorder()
		s.setupRouter().ServeHTTP(rec, loopbackRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	rows := func(body string) int { return strings.Count(body, `id="status-b`) } // one per broker row

	first := get("/api/brokers")
	if got := rows(first); got != brokerPageSize {
		t.Errorf("first page has %d brokers, want %d", got, brokerPageSize)
	}
	if !strings.Contains(first, "Show 50 more") || !strings.Contains(first, `"limit": 100`) {
		t.Error("first page should offer the next 50")
	}
	second := get("/api/brokers?limit=100")
	if got := rows(second); got != 100 || !strings.Contains(second, "Show 20 more") {
		t.Errorf("second page: %d brokers, want 100, and a 'Show 20 more' button", got)
	}
	if all := get("/api/brokers?limit=500"); rows(all) != 120 || strings.Contains(all, "hx-vals") {
		t.Errorf("a limit past the end should show everything and no button")
	}
	if got := rows(get("/api/brokers?limit=-5")); got != brokerPageSize {
		t.Errorf("a bad limit should fall back to one page, got %d", got)
	}
	if page := get("/brokers"); rows(page) != brokerPageSize {
		t.Errorf("the full page should also stop at one page, got %d", rows(page))
	}
}
