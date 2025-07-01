
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pizza-nz/url-shortener/types"
)

// MockURLService is a mock implementation of the URLService interface for testing purposes.
type MockURLService struct {
	CreateShortenedURLFunc func(longURL string) (string, error)
	GetLongURLFunc         func(shortURL string) (string, error)
}

// CreateShortenedURL mocks the CreateShortenedURL method of the URLService interface.
func (m *MockURLService) CreateShortenedURL(longURL string) (string, error) {
	return m.CreateShortenedURLFunc(longURL)
}

// GetLongURL mocks the GetLongURL method of the URLService interface.
func (m *MockURLService) GetLongURL(shortURL string) (string, error) {
	return m.GetLongURLFunc(shortURL)
}

// CountersArr mocks the CountersArr method of the URLService interface.
func (m *MockURLService) CountersArr() []uint64 {
	return []uint64{1, 2}
}

// TestCreateShortenedURL tests the CreateShortenedURL handler function.
func TestCreateShortenedURL(t *testing.T) {
	mockService := &MockURLService{
		CreateShortenedURLFunc: func(longURL string) (string, error) {
			return "shortURL", nil
		},
	}

	handler := NewShortenedURLHandler(mockService)

	// Test case 1: Valid request
	payload := strings.NewReader(`{"longURL": "http://example.com"}`)
	req, err := http.NewRequest("POST", "/shorten", payload)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.CreateShortenedURL(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	expected := `{"shortURL":"/shortURL"}`
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}

	// Test case 2: Invalid request - empty longURL
	payload = strings.NewReader(`{"longURL": ""}`)
	req, err = http.NewRequest("POST", "/shorten", payload)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	handler.CreateShortenedURL(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

// TestGetShortenedURL tests the GetShortenedURL handler function.
func TestGetShortenedURL(t *testing.T) {
	mockService := &MockURLService{
		GetLongURLFunc: func(shortURL string) (string, error) {
			if shortURL == "exists" {
				return "http://example.com", nil
			}
			return "", types.NewAppError("Not Found", "URL not found", http.StatusNotFound, nil)
		},
	}

	handler := NewShortenedURLHandler(mockService)

	// Test case 1: Existing short URL
	req, err := http.NewRequest("GET", "/exists", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("shortURL", "exists")

	rr := httptest.NewRecorder()
	handler.GetShortenedURL(rr, req)

	if status := rr.Code; status != http.StatusMovedPermanently {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMovedPermanently)
	}

	// Test case 2: Non-existing short URL
	req, err = http.NewRequest("GET", "/nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("shortURL", "nonexistent")

	rr = httptest.NewRecorder()
	handler.GetShortenedURL(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}
