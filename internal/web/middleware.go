package web

import (
	"net/http"
	"strings"
)

// Forms are small; cap the body ParseForm may buffer so a broken or hostile
// client cannot make a handler hold an unbounded one in memory.
const maxFormBodyBytes = 1 << 20

// limitFormBody must run before r.ParseForm or r.FormValue. Passing w lets
// MaxBytesReader answer 413 when the client keeps sending past the cap.
func limitFormBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
}

// The CSP allows inline styles and scripts because the layout, HTMX attributes
// and a few page scripts use them; there is no unsafe-eval and every asset is
// served from /static/ (the two font hosts are for the optional web fonts).
var securityHeaderValues = [][2]string{
	{"X-Frame-Options", "DENY"},
	{"X-Content-Type-Options", "nosniff"},
	{"Referrer-Policy", "strict-origin-when-cross-origin"},
	{"Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()"},
	{"Content-Security-Policy", "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"img-src 'self' data:; " +
		"font-src 'self' https://fonts.gstatic.com; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"frame-ancestors 'none'; " +
		"form-action 'self'; " +
		"base-uri 'self'"},
}

// pages can carry credentials and personal data, so they are never cached;
// static assets are left to the file server's own caching
var noCacheHeaderValues = [][2]string{
	{"Cache-Control", "no-store, no-cache, must-revalidate, private"},
	{"Pragma", "no-cache"},
	{"Expires", "0"},
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for _, kv := range securityHeaderValues {
			h.Set(kv[0], kv[1])
		}
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			for _, kv := range noCacheHeaderValues {
				h.Set(kv[0], kv[1])
			}
		}
		next.ServeHTTP(w, r)
	})
}
