package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/google/uuid"
)

var (
	logger                  = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	requestIDKey contextKey = "requestID"
	wg           sync.WaitGroup
	DBURLs       = NewURLMap()
)

type contextKey string

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
		log.Printf("Error: %v", appErr) // Log the detailed error

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTPStatus)
		json.NewEncoder(w).Encode(appErr)
		return
	}

	// For any other error, return a generic 500.
	log.Printf("An unexpected error occurred: %v", err)
	http.Error(w, `{"message":"An internal server error occurred."}`, http.StatusInternalServerError)
}

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

	go http.ListenAndServe(*listenAddr, requestIDMiddleware(mux))
	<-ctx.Done()

	wg.Wait()

	os.Exit(0)
}
