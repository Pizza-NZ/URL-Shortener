package routes

import (
	"log/slog"
	"net/http"
)

// RegisterStaticRoutes registers static routes for the web server.
// This includes the favicon and a root handler.
func RegisterStaticRoutes(mux *http.ServeMux) {
	// Favicon route
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/favicon.ico")
	})

	// Root route
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
		slog.Info("Handled request", "requestID", r.Context().Value(w.Header().Get("X-Request-ID")), "method", r.Method, "url", r.URL.String())
	})
}
