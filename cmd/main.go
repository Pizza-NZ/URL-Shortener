package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/pizza-nz/url-shortener/config"
	"github.com/pizza-nz/url-shortener/middleware"
	"github.com/pizza-nz/url-shortener/routes"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Command-line flag for listening address
	listenAddr := flag.String("listenaddr", ":1232", "Address to listen on")
	flag.Parse()
	slog.Info("Starting server", "listenaddr", *listenAddr)

	// Initilize ServerConfig
	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		slog.Error("Failed to load server configuration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux)

	serverConfig.Server.Addr = *listenAddr
	serverConfig.Server.Handler = middleware.RequestIDMiddleware(mux)

	go serverConfig.MustStart()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	slog.Info("Shutdown signal received, starting graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := serverConfig.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server shutdown gracefully")
	}

	os.Exit(0)
}
