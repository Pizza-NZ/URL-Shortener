package config

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/pizza-nz/url-shortener/types"
)

type DBConfig struct {
	DBConn string `env:"DB_CONN" default:"localhost:5432"` // Database connection string
	DBName string `env:"DB_NAME" default:"url_shortener"`  // Database name
	DBUser string `env:"DB_USER" default:"user"`           // Database user
	DBPass string `env:"DB_PASS" default:"password"`       // Database password
}

func LoadDBConfig() (*DBConfig, error) {
	cfg := &DBConfig{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, types.NewConfigError("Failed to load DB configuration", err)
	}
	return cfg, nil
}

type ServerConfig struct {
	ListenAddr   string `default:":1232"`  // Address to listen on
	ReadTimeout  int    `default:"10000"`  // Read timeout in milliseconds
	WriteTimeout int    `default:"10000"`  // Write timeout in milliseconds
	IdleTimeout  int    `default:"120000"` // Idle timeout in milliseconds

	Server *http.Server `json:"-"` // HTTP server instance
}

func LoadServerConfig() (*ServerConfig, error) {
	cfg := &ServerConfig{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, types.NewConfigError("Failed to load server configuration", err)
	}

	// Initialize the HTTP server with the loaded configuration
	cfg.Server = &http.Server{
		Addr:         cfg.ListenAddr,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Millisecond,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Millisecond,
	}

	return cfg, nil
}

func (cfg *ServerConfig) MustStart() {
	if cfg.Server == nil {
		panic(types.NewConfigError("Server configuration is not initialized", nil))
	}

	slog.Info("Server is starting", "listenaddr", cfg.Server.Addr)
	if err := cfg.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}

func (cfg *ServerConfig) Shutdown(ctx context.Context) error {
	if cfg.Server == nil {
		return types.NewConfigError("Server configuration is not initialized", nil)
	}

	// Shutdown the HTTP server gracefully
	return cfg.Server.Shutdown(ctx)
}
