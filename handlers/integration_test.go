//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pizza-nz/url-shortener/config"
	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/service"
	"github.com/pizza-nz/url-shortener/types"
)

var (
	db  database.Database
	cfg *config.DBConfig
)

func TestMain(m *testing.M) {
	env := os.Getenv("ENV")
	slog.Info("Running integration tests", "environment", env)
	var err error
	cfg, err = config.LoadDBConfig()
	if err != nil {
		panic(err)
	}
	db, err = database.StartNewDatabase(cfg.ConnectionString(), cfg.RedactedConnectionString())
	if err != nil {
		panic(err)
	}

	// Run tests
	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestCreateShortenedURLIntegration(t *testing.T) {
	urlService := service.NewURLService(db)

	mux := http.NewServeMux()
	RegisterAPIRoutesWithMiddleware(mux, urlService)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test case 1: Valid request
	payload := map[string]string{"longURL": "http://example.com"}
		jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", server.URL+"/"+types.APIVersion+"/shorten", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			resp.StatusCode, http.StatusCreated)
	}

	var response map[string]string
	json.NewDecoder(resp.Body).Decode(&response)
	shortURL := response["shortURL"]

	// Test case 2: Existing short URL
	req, _ = http.NewRequest("GET", server.URL+"/"+types.APIVersion+"/shorten/"+shortURL, nil)

	client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("handler returned wrong status code: got %v want %v",
			resp.StatusCode, http.StatusMovedPermanently)
	}
}
