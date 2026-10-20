// Package server builds the HTTP handler of scaleway-manager.
package server

import "net/http"

const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// New returns the handler: GET /healthz and GET / (the given page bytes). Every
// response, 404 and 405 included, carries the security headers.
func New(index []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	return securityHeaders(mux)
}

// securityHeaders sets the headers before the mux runs, so its own 404 and 405
// responses carry them too.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
