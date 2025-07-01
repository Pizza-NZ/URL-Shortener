package middleware

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/types"
	"github.com/pizza-nz/url-shortener/utils"
)

// RequestIDMiddleware is a middleware that generates a unique request ID for each incoming HTTP request.
// It adds the request ID to the response header and logs the request details.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()

		w.Header().Set("X-Request-ID", requestID)
		slog.Info("Received request", "requestID", requestID, "method", r.Method, "url", r.URL.String())

		next.ServeHTTP(w, r)
	})
}

// DBReadyMiddleware checks if the database is connected.
// If not, it returns a 503 Service Unavailable error.
func DBReadyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !database.IsDBReady() {
			utils.HandleError(w, types.NewAppError("Service Not Available", "Database is not ready", http.StatusServiceUnavailable, nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}