package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/google/uuid"
)

type contextKey string

var (
	requestIDKey contextKey = "requestID"
)

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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Command-line flag for listening address
	listenAddr := flag.String("listenaddr", ":1232", "Address to listen on")
	flag.Parse()
	logger.Info("Starting server", "listenaddr", *listenAddr)

	// Set up for graceful shutdown on interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Create a WaitGroup to wait for all reuests to finish before exiting.
	// Need to be careful with context cancellation or ELSE DEADLOCKS
	var wg sync.WaitGroup

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
