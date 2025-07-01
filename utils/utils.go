package utils

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/types"
)

// JSONResponse is a utility function to send a JSON response with the given status code and data.
func JSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err, "requestID", w.Header().Get("X-Request-ID"))
		http.Error(w, `{"message":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

// HandleError is a utility function to handle errors in HTTP handlers.
// It logs the error and sends an appropriate JSON response to the client.
func HandleError(w http.ResponseWriter, err error) {
	var appErr *types.AppError
	if errors.As(err, &appErr) {
		// This is our custom error type, we can trust its fields.
		slog.Error("Handle Error", "Error", appErr) // Log the detailed error

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTPStatus)
		json.NewEncoder(w).Encode(appErr)
		return
	}

	// For any other error, return a generic 500.
	slog.Error("Handle Error", "An unexpected error occurred", err)
	http.Error(w, `{"message":"An internal server error occurred."}`, http.StatusInternalServerError)
}
