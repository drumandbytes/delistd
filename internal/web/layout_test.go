package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func dashboard(t *testing.T, s *Server) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.setupRouter().ServeHTTP(rec, loopbackRequest(http.MethodGet, "/", nil))
	return rec.Body.String()
}

// The nav-link rule (display: inline-flex) outranks .hidden, so without its own
// override the job pill is visible on every page even when nothing is sending.
func TestJobPillStaysHiddenWithoutAJob(t *testing.T) {
	body := dashboard(t, smokeServer(t))
	if !regexp.MustCompile(`id="global-job-indicator"[^>]*class="(?:[^"]* )?hidden[ "]`).MatchString(body) {
		t.Fatal("job indicator should render hidden")
	}
	css, err := staticFS.ReadFile("static/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), ".site-nav-inner a.hidden { display: none; }") {
		t.Error("the hidden override for nav links is missing: the pill would show with no job running")
	}
}
