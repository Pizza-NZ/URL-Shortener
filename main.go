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
	"sync"

	"github.com/google/uuid"
	"github.com/sqids/sqids-go"
)

var (
	logger                  = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	requestIDKey contextKey = "requestID"
	wg           sync.WaitGroup
	DBURLs       = NewURLMap()
	SqidsGen     = NewSqidsGen()
	Counter      = NewGlobalCounter()
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
	Key   string `json:"key"`
	Value string `json:"value"`
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
	wg.Add(1)
	defer wg.Done()

	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, NewBadRequestError([]Details{
			{Field: "body", Issue: "Invalid JSON format"},
		})
	}
	return &payload, nil
}

func JSONResponse(w http.ResponseWriter, status int, data interface{}) {
	wg.Add(1)
	defer wg.Done()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode JSON response", "error", err)
		http.Error(w, `{"message":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

// handle create new shortened URL
func handleCreateShortenedURL(w http.ResponseWriter, r *http.Request) {
	wg.Add(1)
	defer wg.Done()

	if r.Method != http.MethodPost {
		handleError(w, NewAppError("Method Not Allowed", "Only POST method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	key, err := shortenURL(r)
	if err != nil {
		handleError(w, err)
		return
	}

	JSONResponse(w, http.StatusCreated, map[string]string{
		"key": key,
	})
}

// service to handle URL shortening
func shortenURL(r *http.Request) (string, error) {
	wg.Add(1)
	defer wg.Done()

	payload, err := DecodePayload(r)
	if err != nil {
		return "", NewAppError("Failed to decode payload", "Invalid request payload", http.StatusBadRequest, err)
	}

	generatedKey := SqidsGen.Generate()
	if err := DBURLs.Set(generatedKey, payload.Value); err != nil {
		if _, ok := err.(*BadRequestError); ok {
			return "", NewAppError("Bad request", "Invalid input data", http.StatusBadRequest, err)
		}
		return "", NewAppError("Failed to set URL", "Internal server error", http.StatusInternalServerError, err)
	}
	Counter.Increment()
	payload.Key = generatedKey

	return payload.Key, nil

}

// handle get shortened URL
func handleGetShortenedURL(w http.ResponseWriter, r *http.Request) {
	wg.Add(1)
	defer wg.Done()

	if r.Method != http.MethodGet {
		handleError(w, NewAppError("Method Not Allowed", "Only GET method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	key, err := getKey(r)
	if err != nil {
		handleError(w, err)
		return
	}

	longURL, err := serviceGetKeyFromMap(key)
	if err != nil {
		handleError(w, err)
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func serviceGetKeyFromMap(key string) (string, error) {
	wg.Add(1)
	defer wg.Done()

	URL, err := DBURLs.Get(key)
	if err != nil {
		if notFoundErr, ok := err.(*NotFoundError); ok {
			return "", NewAppError("Not Found", notFoundErr.Error(), http.StatusNotFound, err)
		}
		return "", NewAppError("Internal Server Error", "Failed to retrieve URL", http.StatusInternalServerError, err)
	}
	return URL, nil
}

func getKey(r *http.Request) (string, error) {
	wg.Add(1)
	defer wg.Done()

	key := r.URL.Path[1:] // Remove leading slash
	if key == "" {
		return "", NewBadRequestError([]Details{{Field: "key", Issue: "is required"}})
	}
	return key, nil
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

	// Set up for graceful shutdown on interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
		logger.Info("Handled request", "requestID", r.Context().Value(requestIDKey), "method", r.Method, "url", r.URL.String())
	})

	mux.HandleFunc("/shorten", handleCreateShortenedURL)
	mux.HandleFunc("/{key}", handleGetShortenedURL)

	go http.ListenAndServe(*listenAddr, requestIDMiddleware(mux))
	<-ctx.Done()

	wg.Wait()

	os.Exit(0)
}
