package web

import "net/http"

// handleHealthz answers 200 whenever the server is up and the Host check passed.
// It touches neither the config nor the database, so a container health check
// only measures the process (`delistd healthcheck` calls it).
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}
