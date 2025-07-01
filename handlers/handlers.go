package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/pizza-nz/url-shortener/middleware"
	"github.com/pizza-nz/url-shortener/service"
	"github.com/pizza-nz/url-shortener/types"
	"github.com/pizza-nz/url-shortener/utils"
)

// ShortenedURLHandler is an interface that defines methods for handling shortened URLs.
type ShortenedURLHandler interface {
	// CreateShortenedURL handles the creation of a new shortened URL.
	CreateShortenedURL(w http.ResponseWriter, r *http.Request)

	// GetShortenedURL handles the retrieval of a long URL from a shortened URL.
	GetShortenedURL(w http.ResponseWriter, r *http.Request)

	// SetServiceURL sets the URL service for the handler.
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
	Service service.URLService // URL service for URL operations
}

// CreateShortenedURL handles the creation of a new shortened URL.
// It expects a POST request with a JSON payload containing the long URL.
func (h *ShortenedURLHandlerImpl) CreateShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.HandleError(w, types.NewAppError("Method Not Allowed", "Only POST method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	payload, err := types.DecodePayload(r)
	if err != nil {
		utils.HandleError(w, types.NewAppError("Failed to decode payload", "Invalid request payload", http.StatusBadRequest, err))
		return
	}
	if payload.LongURL == "" {
		badRequest := types.NewBadRequestError([]types.Details{types.NewDetails("LongURL", "Long URL cannot be empty")})
		utils.HandleError(w, types.NewAppError("Bad Request", badRequest.Error(), http.StatusBadRequest, badRequest))
		return
	}

	// Check if service is nil, if so return 503
	if h.Service == nil {
		utils.HandleError(w, types.NewAppError("Service Unavailable", "DB is not set up", http.StatusServiceUnavailable, nil))
		return
	}

	shortURL, err := h.Service.CreateShortenedURL(payload.LongURL)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.JSONResponse(w, http.StatusCreated, map[string]string{
		"shortURL": shortURL,
	})

}

// GetShortenedURL handles the retrieval of a long URL from a shortened URL.
// It redirects the user to the long URL associated with the provided short URL.
// If the short URL does not exist, it returns a 404 Not Found error.
func (h *ShortenedURLHandlerImpl) GetShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.HandleError(w, types.NewAppError("Method Not Allowed", "Only GET method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	shortURL := strings.TrimPrefix(r.URL.Path, "/"+types.APIVersion+"/shorten/")

	// Protection from panic if Service is nil
	if h.Service == nil {
		utils.HandleError(w, types.NewAppError("Internal Server Error", "service var is nil", http.StatusInternalServerError, nil))
		return
	}

	longURL, err := h.Service.GetLongURL(shortURL)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
	slog.Info("Redirecting to long URL", "shortURL", shortURL, "longURL", longURL, "requestID", w.Header().Get("X-Request-ID"))
}

// SetServiceURL sets the URL service for the handler.
func (h *ShortenedURLHandlerImpl) SetServiceURL(service service.URLService) {
	h.Service = service
}

// RegisterAPIRoutesWithMiddleware registers API routes for the URL shortening service with middlewares.
// It sets up routes for creating and retrieving shortened URLs, with a database readiness check.
func RegisterAPIRoutesWithMiddleware(mux *http.ServeMux, service service.URLService) ShortenedURLHandler {
	// ShortenedURLHandler
	shortenedURLHandler := NewShortenedURLHandler(service)

	// API route for creating a shortened URL
	mux.Handle("/"+types.APIVersion+"/shorten", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.CreateShortenedURL)))

	// API route for retrieving a long URL from a shortened URL
	mux.Handle("/"+types.APIVersion+"/shorten/", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.GetShortenedURL)))

	return shortenedURLHandler
}
