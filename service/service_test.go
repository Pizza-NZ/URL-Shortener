package service

import (
	"testing"

	
	"github.com/pizza-nz/url-shortener/types"
)

// MockDatabase is a mock implementation of the Database interface for testing purposes.
type MockDatabase struct {
	GetFunc func(key string) (string, error)
	SetFunc func(key, value string) error
}

func (m *MockDatabase) Get(key string) (string, error) {
	return m.GetFunc(key)
}

func (m *MockDatabase) Set(key, value string) error {
	return m.SetFunc(key, value)
}

func (m *MockDatabase) GetAndIncreament() (uint64, error) {
	return 1, nil
}

func TestCreateShortenedURL(t *testing.T) {
	mockDB := &MockDatabase{
		SetFunc: func(key, value string) error {
			return nil
		},
	}

	service := NewURLService(mockDB)

	longURL := "http://example.com"
	shortURL, err := service.CreateShortenedURL(longURL)

	if err != nil {
		t.Errorf("CreateShortenedURL() error = %v, wantErr nil", err)
	}

	if shortURL == "" {
		t.Error("Expected a shortURL, but got an empty string")
	}
}

func TestGetLongURL(t *testing.T) {
	mockDB := &MockDatabase{
		GetFunc: func(key string) (string, error) {
			if key == "exists" {
				return "http://example.com", nil
			}
			return "", types.NewNotFoundError("not found")
		},
	}

	service := NewURLService(mockDB)

	// Test case 1: Existing short URL
	longURL, err := service.GetLongURL("exists")
	if err != nil {
		t.Errorf("GetLongURL() error = %v, wantErr nil", err)
	}

	if longURL != "http://example.com" {
		t.Errorf("GetLongURL() = %v, want %v", longURL, "http://example.com")
	}

	// Test case 2: Non-existing short URL
	_, err = service.GetLongURL("nonexistent")
	if err == nil {
		t.Error("Expected an error for non-existent short URL, but got nil")
	}
}

func TestMain(m *testing.M) {
	isInit = true
	m.Run()
}
