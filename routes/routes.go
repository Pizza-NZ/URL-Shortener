package routes

import (
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/handlers"
	"github.com/pizza-nz/url-shortener/middleware"
	"github.com/pizza-nz/url-shortener/service"
	"github.com/pizza-nz/url-shortener/types"
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

// RegisterAPIRoutes registers API routes for the URL shortening service.
// This function is deprecated and should not be used.
// Use RegisterAPIRoutesWithMiddleware instead.
func RegisterAPIRoutes(mux *http.ServeMux, service service.URLService) handlers.ShortenedURLHandler {
	// ShortenedURLHandler
	shortenedURLHandler := handlers.NewShortenedURLHandler(service)

	// API route for creating a shortened URL
	mux.HandleFunc("/"+types.APIVersion+"/shorten", shortenedURLHandler.CreateShortenedURL)

	// API route for retrieving a long URL from a shortened URL
	mux.HandleFunc("/"+types.APIVersion+"/shorten/{shortURL}", shortenedURLHandler.GetShortenedURL)

	return shortenedURLHandler
}

// RegisterAPIRoutesWithMiddleware registers API routes for the URL shortening service with middlewares.
// It sets up routes for creating and retrieving shortened URLs, with a database readiness check.
func RegisterAPIRoutesWithMiddleware(mux *http.ServeMux, service service.URLService) handlers.ShortenedURLHandler {
	// ShortenedURLHandler
	shortenedURLHandler := handlers.NewShortenedURLHandler(service)

	// API route for creating a shortened URL
	mux.Handle("/"+types.APIVersion+"/shorten", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.CreateShortenedURL)))

	// API route for retrieving a long URL from a shortened URL
	mux.Handle("/"+types.APIVersion+"/shorten/{shortURL}", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.GetShortenedURL)))

	return shortenedURLHandler
}