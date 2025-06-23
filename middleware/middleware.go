package middleware

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDMiddleware is a middleware that generates a unique request ID for each incoming HTTP request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()

		w.Header().Set("X-Request-ID", requestID)
		slog.Info("Received request", "requestID", requestID, "method", r.Method, "url", r.URL.String())

		next.ServeHTTP(w, r)
	})
}
