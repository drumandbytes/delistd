package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzAnswersWithoutTouchingConfig(t *testing.T) {
	s := smokeServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Host = "127.0.0.1:8080"
	s.setupRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "ok") {
		t.Errorf("healthz = %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Host = "evil.example.com"
	rec = httptest.NewRecorder()
	s.setupRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a foreign Host got %d from /healthz, want 403", rec.Code)
	}
}
