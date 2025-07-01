package service

import (
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/types"
)

// URLService is an interface for the URL shortening service.
// It defines methods for creating and retrieving shortened URLs.
type URLService interface {
	// CreateShortenedURL creates a new shortened URL from a long URL.
	CreateShortenedURL(longURL string) (string, error)

	// GetLongURL retrieves the long URL associated with a given shortened URL.
	GetLongURL(shortURL string) (string, error)
}

// URLServiceImpl is a concrete implementation of the URLService interface.
// It uses a database for URL storage and a Sqids generator for creating short URLs.
type URLServiceImpl struct {
	DBURLs   database.Database // Database for storing URLs
	SqidsGen *types.SqidsGen   // Sqids generator for creating short URLs
}

// NewURLService creates a new instance of URLService.
// It initializes the URLServiceImpl with a database and a SqidsGen.
func NewURLService(db database.Database) URLService {
	return &URLServiceImpl{
		DBURLs:   db,
		SqidsGen: types.NewSqidsGen(),
	}
}

// CreateShortenedURL creates a new shortened URL from a long URL.
// It generates a short URL, stores it in the database, and returns the short URL.
func (s *URLServiceImpl) CreateShortenedURL(longURL string) (string, error) {
	shortURL := s.SqidsGen.Generate(s.CountersArr())
	if err := s.DBURLs.Set(shortURL, longURL); err != nil {
		if _, ok := err.(*types.BadRequestError); ok {
			return "", types.NewAppError("Bad request", "Invalid input data", http.StatusBadRequest, err)
		}
		return "", types.NewAppError("Failed to set URL", "Internal server error", http.StatusInternalServerError, err)
	}
	slog.Info("Shortened URL created", "shortURL", shortURL, "longURL", longURL)

	return shortURL, nil
}

// GetLongURL retrieves the long URL associated with a given shortened URL.
// It fetches the URL from the database and returns it.
func (s *URLServiceImpl) GetLongURL(shortURL string) (string, error) {
	URL, err := s.DBURLs.Get(shortURL)
	if err != nil {
		if _, ok := err.(*types.NotFoundError); ok {
			return "", types.NewAppError("Not Found", "Service failed to get URL from map", http.StatusNotFound, err)
		}
		return "", types.NewAppError("Internal Server Error", "Failed to retrieve URL", http.StatusInternalServerError, err)
	}
	return URL, nil
}