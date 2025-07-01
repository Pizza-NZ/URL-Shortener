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
	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/handlers"
	"github.com/pizza-nz/url-shortener/middleware"
	"github.com/pizza-nz/url-shortener/routes"
	"github.com/pizza-nz/url-shortener/service"
)

// MainConfig holds the top-level configuration for the application,
// aggregating server and database settings.
type MainConfig struct {
	serverCfg *config.ServerConfig
	dbCfg     *config.DBConfig
}

// cfg is a package-level variable holding the application's configuration.
var cfg MainConfig

// mustInitConfig initializes the server and database configurations.
// It panics if loading the configuration fails, ensuring the application
// does not start with invalid settings.
func mustInitConfig() {
	// Initialize ServerConfig
	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		slog.Error("Failed to load server configuration", "error", err)
		os.Exit(1)
	}

	// Initialize DBConfig
	DBConfig, err := config.LoadDBConfig()
	if err != nil {
		slog.Error("Failed to load database configuration", "error", err)
		os.Exit(1)
	}

	cfg = MainConfig{
		serverCfg: serverConfig,
		dbCfg:     DBConfig,
	}
}

// connectWithRetry attempts to connect to the database with a retry mechanism.
// It tries to connect every 10 seconds for up to 1 minute. If the connection
// is successful, it sets the URL service for the handler.
func connectWithRetry(handler handlers.ShortenedURLHandler) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	tickerAttempt := 1

	timeout := time.After(1 * time.Minute)

	slog.Info("Starting database connection attempts")

	var lastErr error
	for {
		select {
		case <-timeout:
			slog.Error("connectWithRetry Failed to connect to the database after 1 minute.", "last error", lastErr)
			return
		case <-ticker.C:
			conn, err := database.StartNewDatabase(cfg.dbCfg.ConnectionString())
			if err != nil {
				slog.Warn("connectWithRetry Failed to connect to the database, retrying...", "Attempt", tickerAttempt, "Error", err)
				lastErr = err
				tickerAttempt++
				continue
			}

			handler.SetServiceURL(service.NewURLService(conn))

			slog.Info("connectWithRetry connected successfully", "Total Attempts", tickerAttempt)
			return
		}
	}
}

// main is the entry point of the application.
// It initializes the logger, configuration, routes, and starts the server.
// It also handles graceful shutdown on receiving an interrupt signal.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Command-line flag for listening address
	listenAddr := flag.String("listenaddr", ":1232", "Address to listen on")
	flag.Parse()
	slog.Info("Starting server", "listenaddr", *listenAddr)

	mustInitConfig()

	mux := http.NewServeMux()
	routes.RegisterStaticRoutes(mux)
	handler := routes.RegisterAPIRoutesWithMiddleware(mux, nil)

	go connectWithRetry(handler)

	cfg.serverCfg.Server.Addr = *listenAddr
	cfg.serverCfg.Server.Handler = middleware.RequestIDMiddleware(mux)

	go cfg.serverCfg.MustStart()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	slog.Info("Shutdown signal received, starting graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := cfg.serverCfg.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server shutdown gracefully")
	}

	os.Exit(0)
}