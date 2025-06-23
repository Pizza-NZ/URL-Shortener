package database

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/pizza-nz/url-shortener/types"
)

type Database interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// DatabaseURLMapImpl is a thread-safe map for storing URLs with their corresponding short keys.
type DatabaseURLMapImpl struct {
	lock sync.RWMutex
	URLs map[string]string
}

// NewDatabaseURLMapImpl creates a new instance of DatabaseURLMapImpl.
// It initializes the internal map to ensure it is ready for use.
func NewDatabaseURLMapImpl() Database {
	return &DatabaseURLMapImpl{
		URLs: make(map[string]string),
	}
}

// Get retrieves the value associated with the given key from the URLMap.
// If the key does not exist, it returns a NotFoundError.
// It uses a read lock to ensure thread safety during the retrieval.
// The returned value is the long URL associated with the short key.
func (m *DatabaseURLMapImpl) Get(key string) (string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	value, exists := m.URLs[key]
	if !exists {
		return "", types.NewNotFoundError(key)
	}
	return value, nil
}

// Set adds a new key-value pair to the URLMap.// It checks if the key and value are not empty and if the key does not already exist.
// // If any of these conditions are not met, it returns a BadRequestError with appropriate details.
// // It uses a write lock to ensure thread safety during the addition.
// If successful, it logs the addition of the URL to the map.
// If the key already exists, it returns a BadRequestError indicating that the key is already
func (m *DatabaseURLMapImpl) Set(key, value string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	details := []types.Details{}
	if key == "" {
		details = append(details, types.Details{Field: "key", Issue: "cannot be empty"})
	}
	if value == "" {
		details = append(details, types.Details{Field: "value", Issue: "cannot be empty"})
	}
	if len(details) > 0 {
		return types.NewBadRequestError(details)
	}
	if _, exists := m.URLs[key]; exists {
		details = append(details, types.Details{Field: "key", Issue: fmt.Sprintf("key '%s' already exists", key)})
		return types.NewBadRequestError(details)
	}

	m.URLs[key] = value
	slog.Info("URL added to map", "key", key, "value", value)

	return nil
}
