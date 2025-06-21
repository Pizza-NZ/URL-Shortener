package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sqids/sqids-go"
)

var (
	logger                  = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	requestIDKey contextKey = "requestID"
	DBURLs                  = NewURLMap()
	SqidsGen                = NewSqidsGen()
	Counter                 = NewGlobalCounter()
)

type GlobalCounter struct {
	mu    sync.Mutex
	count uint64
}

func (c *GlobalCounter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func (c *GlobalCounter) Count() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func NewGlobalCounter() *GlobalCounter {
	return &GlobalCounter{
		count: 0,
	}
}

type sqidsGen struct {
	Sqid *sqids.Sqids
}

func NewSqidsGen() *sqidsGen {
	squid, _ := sqids.New()
	sqidsGen := &sqidsGen{
		Sqid: squid,
	}
	return sqidsGen
}

func (s *sqidsGen) Generate() string {
	id, _ := s.Sqid.Encode([]uint64{Counter.Count()})
	return id
}

type contextKey string

type Payload struct {
	ShortURL string `json:"ShortURL"`
	LongURL  string `json:"LongURL"`
}

type URLMap struct {
	lock sync.RWMutex
	URLs map[string]string
}

func NewURLMap() *URLMap {
	return &URLMap{
		URLs: make(map[string]string),
	}
}

type NotFoundError struct {
	key string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("The requested key (%s) does not exist in the map.", e.key)
}

func NewNotFoundError(key string) *NotFoundError {
	return &NotFoundError{key: key}
}

type Details struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

type BadRequestError struct {
	Details []Details `json:"details"`
}

func (e *BadRequestError) Error() string {
	if len(e.Details) == 0 {
		return "Bad request with no details provided."
	}
	var issues []string
	for _, detail := range e.Details {
		issues = append(issues, fmt.Sprintf("%s: %s", detail.Field, detail.Issue))
	}
	return fmt.Sprintf("Bad request with issues: %s", issues)
}

func NewBadRequestError(details []Details) *BadRequestError {
	return &BadRequestError{
		Details: details,
	}
}

type AppError struct {
	Underlying error `json:"-"`

	HTTPStatus int `json:"-"`

	Message string `json:"message"`

	InternalMessage string `json:"-"`
}

func (e *AppError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("message: %s, internal_message: %s, underlying_error: %v", e.Message, e.InternalMessage, e.Underlying)
	}
	return fmt.Sprintf("message: %s, internal_message: %s", e.Message, e.InternalMessage)
}

func (e *AppError) Unwrap() error {
	return e.Underlying
}

func NewAppError(message, internalMessage string, httpStatus int, underlying error) *AppError {
	return &AppError{
		Message:         message,
		InternalMessage: internalMessage,
		HTTPStatus:      httpStatus,
		Underlying:      underlying,
	}
}

func (m *URLMap) Get(key string) (string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	value, exists := m.URLs[key]
	if !exists {
		return "", NewNotFoundError(key)
	}
	return value, nil
}

func (m *URLMap) Set(key, value string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	details := []Details{}
	if key == "" {
		details = append(details, Details{Field: "key", Issue: "cannot be empty"})
	}
	if value == "" {
		details = append(details, Details{Field: "value", Issue: "cannot be empty"})
	}
	if len(details) > 0 {
		return NewBadRequestError(details)
	}
	if _, exists := m.URLs[key]; exists {
		details = append(details, Details{Field: "key", Issue: fmt.Sprintf("key '%s' already exists", key)})
		return NewBadRequestError(details)
	}

	m.URLs[key] = value
	slog.Info("URL added to map", "key", key, "value", value)

	return nil
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", requestID)
		slog.Info("Received request", "requestID", requestID, "method", r.Method, "url", r.URL.String())

		next.ServeHTTP(w, r)
	})
}

func handleError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		// This is our custom error type, we can trust its fields.
		logger.Error("Handle Error", "Error", appErr) // Log the detailed error

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTPStatus)
		json.NewEncoder(w).Encode(appErr)
		return
	}

	// For any other error, return a generic 500.
	logger.Error("Handle Error", "An unexpected error occurred", err)
	http.Error(w, `{"message":"An internal server error occurred."}`, http.StatusInternalServerError)
}

// DecodePayload decodes the JSON payload from the request body
func DecodePayload(r *http.Request) (*Payload, error) {
	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, NewBadRequestError([]Details{
			{Field: "body", Issue: "Invalid JSON format"},
		})
	}
	return &payload, nil
}

func JSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode JSON response", "error", err)
		http.Error(w, `{"message":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

// handle create new shortened URL
func handleCreateShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		handleError(w, NewAppError("Method Not Allowed", "Only POST method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	shortURL, err := serviceToShortenURL(r)
	if err != nil {
		handleError(w, err)
		return
	}

	JSONResponse(w, http.StatusCreated, map[string]string{
		"shortURL": fmt.Sprintf("%s%s", r.URL.Path, shortURL),
	})
}

// service to handle URL shortening
func serviceToShortenURL(r *http.Request) (string, error) {
	payload, err := DecodePayload(r)
	if err != nil {
		return "", NewAppError("Failed to decode payload", "Invalid request payload", http.StatusBadRequest, err)
	}

	generatedShortURL := SqidsGen.Generate()
	if err := DBURLs.Set(generatedShortURL, payload.LongURL); err != nil {
		if _, ok := err.(*BadRequestError); ok {
			return "", NewAppError("Bad request", "Invalid input data", http.StatusBadRequest, err)
		}
		return "", NewAppError("Failed to set URL", "Internal server error", http.StatusInternalServerError, err)
	}
	Counter.Increment()
	payload.ShortURL = generatedShortURL
	logger.Info("Shortened URL created", "shortURL", payload.ShortURL, "longURL", payload.LongURL)

	return payload.ShortURL, nil
}

// handle get shortened URL
func handleGetShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		handleError(w, NewAppError("Method Not Allowed", "Only GET method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	shortURL, err := getShortenURL(r)
	if err != nil {
		handleError(w, err)
		return
	}

	longURL, err := serviceGetShortenURLFromMap(shortURL)
	if err != nil {
		handleError(w, err)
		return
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
	logger.Info("Redirecting to long URL", "shortURL", shortURL, "longURL", longURL, "requestID", r.Context().Value(requestIDKey))
}

func serviceGetShortenURLFromMap(shortURL string) (string, error) {
	URL, err := DBURLs.Get(shortURL)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			return "", NewAppError("Not Found", "Service failed to get URL from map", http.StatusNotFound, err)
		}
		return "", NewAppError("Internal Server Error", "Failed to retrieve URL", http.StatusInternalServerError, err)
	}
	return URL, nil
}

func getShortenURL(r *http.Request) (string, error) {
	shortURL := strings.TrimPrefix(r.URL.Path, "/") // Remove leading slash
	if shortURL == "" {
		return "", NewBadRequestError([]Details{{Field: "shortURL", Issue: "is required"}})
	}
	return shortURL, nil
}

// handle delete shortened URL

// handle update shortened URL

// handle redirect from shortened URL to original URL

func main() {
	slog.SetDefault(logger)

	// Command-line flag for listening address
	listenAddr := flag.String("listenaddr", ":1232", "Address to listen on")
	flag.Parse()
	logger.Info("Starting server", "listenaddr", *listenAddr)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
		logger.Info("Handled request", "requestID", r.Context().Value(requestIDKey), "method", r.Method, "url", r.URL.String())
	})

	mux.HandleFunc("/shorten", handleCreateShortenedURL)
	mux.HandleFunc("/{shortURL}", handleGetShortenedURL)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/favicon.ico")
		logger.Info("Served favicon", "requestID", r.Context().Value(requestIDKey), "method", r.Method, "url", r.URL.String())
	})

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: requestIDMiddleware(mux),
	}

	go func() {
		slog.Info("Server is starting", "listenaddr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	slog.Info("Shutdown signal received, starting graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server shutdown gracefully")
	}

	os.Exit(0)
}
