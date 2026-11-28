package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drumandbytes/delistd/internal/config"
)

// With an inbox configured the pipeline page offers its three mailbox
// actions, each posting to its endpoint; the two long-running ones point at
// a busy indicator that exists on the page.
func TestPipelineMailboxButtons(t *testing.T) {
	s := smokeServer(t)
	cfg := s.getConfig()
	cfg.Inbox = config.InboxConfig{Enabled: true, Server: "127.0.0.1", Port: 1, Email: "test@example.com", Password: "x"}
	s.config.Store(cfg)

	rec := httptest.NewRecorder()
	s.setupRouter().ServeHTTP(rec, loopbackRequest(http.MethodGet, "/pipeline", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`hx-post="/api/inbox/scan"`, `hx-post="/api/inbox/reclassify"`, `hx-post="/api/inbox/bounces"`,
		`hx-indicator="#scan-spinner"`, `id="scan-spinner"`, `hx-indicator="#reclassify-spinner"`, `id="reclassify-spinner"`,
		"Scan Inbox", "Reclassify", "Check bounces", `a "Delistd" folder`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pipeline page lacks %q", want)
		}
	}
	if strings.Contains(body, `an "Delistd"`) {
		t.Error("the old 'an \"Delistd\"' wording is back")
	}
}
