package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/pizza-nz/url-shortener/types"
)

// DBConfig holds the configuration for the database connection.
// It includes host, port, name, user, and password.
type DBConfig struct {
	DBHost string `default:"localhost:5432"`
	DBPort string `default:"5432"`
	DBName string `default:"url_shortener"` // Database name
	DBUser string `default:"user"`          // Database user
	DBPass string `default:"password"`      // Database password
}

// LoadDBConfig loads the database configuration from environment variables.
// It returns a DBConfig instance or an error if loading fails.
func LoadDBConfig() (*DBConfig, error) {
	cfg := &DBConfig{}
	// if err := envconfig.Process("", cfg); err != nil {
	// 	return nil, types.NewConfigError("Failed to load DB configuration", err)
	// }
	cfg.DBHost = os.Getenv("DB_HOST")
	cfg.DBPort = os.Getenv("DB_PORT")
	cfg.DBName = os.Getenv("DB_NAME")
	cfg.DBUser = os.Getenv("DB_USER")
	cfg.DBPass = os.Getenv("DB_PASS")

	return cfg, nil
}

// ConnectionString returns the formatted connection string for the database.
func (cfg *DBConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
}

// RedactedConnectionString returns the formatted connection string for the database with the password redacted.
func (cfg *DBConfig) RedactedConnectionString() string {
	return fmt.Sprintf("postgres://%s:xxxxx@%s:%s/%s?sslmode=disable", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)
}

// ServerConfig holds the configuration for the HTTP server.
// It includes listen address, timeouts, and the server instance itself.
type ServerConfig struct {
	ListenAddr   string `env:"LISTENADDR" default:":1232"`   // Address to listen on
	ReadTimeout  int    `env:"READTIMEOUT" default:"10000"`  // Read timeout in milliseconds
	WriteTimeout int    `env:"WRITETIMEOUT" default:"10000"` // Write timeout in milliseconds
	IdleTimeout  int    `env:"IDLETIMEOUT" default:"120000"` // Idle timeout in milliseconds

	Server *http.Server `json:"-"` // HTTP server instance
}

// LoadServerConfig loads the server configuration from environment variables.
// It initializes the HTTP server with the loaded settings.
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

// MustStart starts the HTTP server.
// It panics if the server configuration is not initialized or if the server fails to start.
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

// Shutdown gracefully shuts down the HTTP server.
// It returns an error if the server configuration is not initialized.
func (cfg *ServerConfig) Shutdown(ctx context.Context) error {
	if cfg.Server == nil {
		return types.NewConfigError("Server configuration is not initialized", nil)
	}

	// Shutdown the HTTP server gracefully
	return cfg.Server.Shutdown(ctx)
}