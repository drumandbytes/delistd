package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPagesAreServedAsHTML(t *testing.T) {
	s := smokeServer(t)
	rec := httptest.NewRecorder()
	s.setupRouter().ServeHTTP(rec, loopbackRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

// A template that fails halfway must not leave half a page in front of the
// error: nothing is sent until the whole page has rendered.
func TestFailedRenderSendsOnlyTheError(t *testing.T) {
	tmpl := template.Must(template.New("x").Parse(`<p>before</p>{{index .Missing 3}}<p>after</p>`))
	rec := httptest.NewRecorder()
	writeHTML(rec, tmpl, "", map[string]any{"Missing": []int{}})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "before") {
		t.Errorf("partial output leaked: %q", rec.Body.String())
	}
}

func TestEveryTemplateParses(t *testing.T) {
	set, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dashboard.html", "brokers.html", "partials/broker-list.html", "setup/welcome.html"} {
		if set[name] == nil {
			t.Errorf("template %s missing", name)
		}
	}
}

func TestRateLimiterWindow(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	for i := 0; i < 2; i++ {
		if !rl.Allow("a") {
			t.Fatalf("hit %d is within the limit", i+1)
		}
	}
	if rl.Allow("a") {
		t.Error("a third hit inside the window must be refused")
	}
	if !rl.Allow("b") {
		t.Error("keys are limited separately")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.Allow("a") {
		t.Error("the window slides: a hit is allowed once the old ones have left it")
	}
}
