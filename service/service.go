package service

import (
	"log/slog"
	"net/http"

	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/types"
)

type URLService interface {
	// CreateShortenedURL creates a new shortened URL from a long URL.
	CreateShortenedURL(longURL string) (string, error)

	// GetLongURL retrieves the long URL associated with a given shortened URL.
	GetLongURL(shortURL string) (string, error)
}

type URLServiceImpl struct {
	DBURLs   database.Database // URLMap to store URLs
	SqidsGen *types.SqidsGen   // Sqids generator for creating short URLs
}

func NewURLService() URLService {
	// Initialize the URLServiceImpl with a URLMap and SqidsGen.
	return &URLServiceImpl{
		DBURLs:   database.NewDatabaseURLMapImpl(),
		SqidsGen: types.NewSqidsGen(),
	}
}

func (s *URLServiceImpl) CreateShortenedURL(longURL string) (string, error) {
	shortURL := s.SqidsGen.Generate()
	if err := s.DBURLs.Set(shortURL, longURL); err != nil {
		if _, ok := err.(*types.BadRequestError); ok {
			return "", types.NewAppError("Bad request", "Invalid input data", http.StatusBadRequest, err)
		}
		return "", types.NewAppError("Failed to set URL", "Internal server error", http.StatusInternalServerError, err)
	}
	slog.Info("Shortened URL created", "shortURL", shortURL, "longURL", longURL)

	return shortURL, nil
}

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
