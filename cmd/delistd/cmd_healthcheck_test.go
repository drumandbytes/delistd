package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer ok.Close()
	if err := checkHealth(context.Background(), strings.TrimPrefix(ok.URL, "http://")); err != nil {
		t.Errorf("healthy server: %v", err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusServiceUnavailable) }))
	defer broken.Close()
	if err := checkHealth(context.Background(), strings.TrimPrefix(broken.URL, "http://")); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("503 = %v", err)
	}

	gone := httptest.NewServer(http.NotFoundHandler())
	addr := strings.TrimPrefix(gone.URL, "http://")
	gone.Close()
	if err := checkHealth(context.Background(), addr); err == nil {
		t.Error("a closed port reported healthy")
	}
}
