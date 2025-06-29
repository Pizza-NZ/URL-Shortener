package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/service"
	"github.com/pizza-nz/url-shortener/types"
)

// ShortenedURLHandler is an interface that defines methods for handling shortened URLs.
type ShortenedURLHandler interface {
	// CreateShortenedURL handles the creation of a new shortened URL.
	CreateShortenedURL(w http.ResponseWriter, r *http.Request)

	// GetShortenedURL handles the retrieval of a long URL from a shortened URL.
	GetShortenedURL(w http.ResponseWriter, r *http.Request)

	SetServiceURL(service service.URLService)
}

// NewShortenedURLHandler creates a new instance of ShortenedURLHandler.
// It initializes the handler with the necessary services or dependencies.
func NewShortenedURLHandler(service service.URLService) ShortenedURLHandler {
	return &ShortenedURLHandlerImpl{
		Service: service, // Assuming you have a service constructor
	}
}

// ShortenedURLHandlerImpl is a concrete implementation of the ShortenedURLHandler interface.
type ShortenedURLHandlerImpl struct {
	Service service.URLService // Assuming you have a service interface for URL operations
}

// CreateShortenedURL handles the creation of a new shortened URL.
func (h *ShortenedURLHandlerImpl) CreateShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		HandleError(w, types.NewAppError("Method Not Allowed", "Only POST method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	payload, err := types.DecodePayload(r)
	if err != nil {
		HandleError(w, types.NewAppError("Failed to decode payload", "Invalid request payload", http.StatusBadRequest, err))
		return
	}
	if payload.LongURL == "" {
		badRequest := types.NewBadRequestError([]types.Details{types.NewDetails("LongURL", "Long URL cannot be empty")})
		HandleError(w, types.NewAppError("Bad Request", badRequest.Error(), http.StatusBadRequest, badRequest))
		return
	}

	// Check if service nil, if nil then send 503
	if h.Service == nil {
		HandleError(w, types.NewAppError("Service Unavaible", "DB is not set up", 503, nil))
		return
	}

	shortURL, err := h.Service.CreateShortenedURL(payload.LongURL)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSONResponse(w, http.StatusCreated, map[string]string{
		"shortURL": "/" + shortURL,
	})

}

// GetShortenedURL handles the retrieval of a long URL from a shortened URL.
// It redirects the user to the long URL associated with the provided short URL.
// If the short URL does not exist, it returns a 404 Not Found error.
func (h *ShortenedURLHandlerImpl) GetShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		HandleError(w, types.NewAppError("Method Not Allowed", "Only GET method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	shortURL := r.PathValue("shortURL")

	// Protection from panic if Service is nil
	if h.Service == nil {
		HandleError(w, types.NewAppError("Internal Server Error", "service var is nil", 500, nil))
		return
	}

	longURL, err := h.Service.GetLongURL(shortURL)
	if err != nil {
		HandleError(w, err)
		return
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
	slog.Info("Redirecting to long URL", "shortURL", shortURL, "longURL", longURL, "requestID", w.Header().Get("X-Request-ID"))
}

// SetServiceURL
func (h *ShortenedURLHandlerImpl) SetServiceURL(service service.URLService) {
	h.Service = service
}

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
