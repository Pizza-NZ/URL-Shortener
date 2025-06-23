package routes

import (
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/handlers"
	"github.com/pizza-nz/url-shortener/types"
)

// RegisterRoutes registers all routes for the server. Basic entry point when setting up server.
func RegisterRoutes(mux *http.ServeMux) {
	RegisterStaticRoutes(mux)

	RegisterAPIRoutes(mux)
}

// RegisterStaticRoutes registers static routes for the web server.
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

// RegisterAPIRoutes registers API routes for the URL shortening service.
func RegisterAPIRoutes(mux *http.ServeMux) {
	// ShortenedURLHandler
	shortenedURLHandler := handlers.NewShortenedURLHandler()

	// API route for creating a shortened URL
	mux.HandleFunc("/"+types.APIVersion+"/shorten", shortenedURLHandler.CreateShortenedURL)

	// API route for retrieving a long URL from a shortened URL
	mux.HandleFunc("/"+types.APIVersion+"/shorten/{shortURL}", shortenedURLHandler.GetShortenedURL)
}
