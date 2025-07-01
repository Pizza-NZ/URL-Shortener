# URL Shortener Go Application Documentation

## 1. Overview

This document provides a detailed explanation of a URL shortener web service written in Go. The application is designed to take a long URL and generate a unique, shorter URL. When a user accesses the short URL, they are redirected to the original long URL.

The service is built with a modular architecture, separating concerns like configuration, database interaction, business logic (service layer), and request handling. It supports both an in-memory data store for simple use cases and a PostgreSQL database for persistent storage.

## 2. Core Features

*   **URL Shortening:** Creates a unique short ID for any given long URL.
*   **URL Redirection:** Redirects users from the short URL to the original long URL.
*   **Dual Database Support:**
    *   **In-Memory:** A thread-safe map for quick, non-persistent storage.
    *   **PostgreSQL:** A persistent database solution with connection pooling and migrations.
*   **Unique ID Generation:** Uses the `sqids` library combined with a dual-counter system (one in-memory, one in-database) to generate unique, non-sequential short IDs.
*   **RESTful API:** Provides a simple JSON-based API for creating short URLs.
*   **Structured Error Handling:** Implements a robust system of custom error types for clear and consistent error responses.
*   **Middleware:** Includes middleware for adding a unique Request ID to each request for logging and tracing.

## 3. Project Structure

The project is organized into the following directories:

*   `cmd/main.go`: The main application entry point.
*   `config/`: Handles loading environment variables for server and database configuration.
*   `database/`: Manages database connections (PostgreSQL and in-memory) and migrations.
*   `handlers/`: Contains HTTP handlers that process incoming requests.
*   `middleware/`: Provides HTTP middleware for request processing.
*   `routes/`: Defines the application's URL routes and maps them to handlers.
*   `service/`: Implements the core business logic of the application.
*   `types/`: Defines shared data structures, payloads, and custom error types.

## 4. Key Components & Code

### 4.1. Configuration (`config/config.go`)

The application loads configuration from environment variables into structs.

*   **`DBConfig`**: Holds database connection details.
*   **`ServerConfig`**: Holds HTTP server settings like address and timeouts.

```go
// DBConfig holds the configuration for the database connection.
// It includes host, port, name, user, and password.
type DBConfig struct {
	DBHost string `default:"localhost:5432"`
	DBPort string `default:"5432"`
	DBName string `default:"url_shortener"` // Database name
	DBUser string `default:"user"`          // Database user
	DBPass string `default:"password"`      // Database password
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
```

### 4.2. Database Layer (`database/database.go`)

The database layer is abstracted through a `Database` interface, allowing for interchangeable storage backends.

```go
// Database is an interface for URL storage.
// It defines methods for getting and setting URL data.
type Database interface {
	Get(key string) (string, error)
	Set(key, value string) error
}
```

Two implementations are provided:
1.  `DatabaseURLMapImpl`: An in-memory, thread-safe map.
2.  `DatabaseURLPGImpl`: A PostgreSQL implementation using `pgxpool`.

The `StartNewDatabase` function acts as a factory, returning the correct database type based on the provided connection string.

### 4.3. Service Layer & ID Generation (`service/service.go`, `service/counter.go`)

The core logic resides in the `URLServiceImpl`. Its primary job is to create and retrieve URLs.

To create a unique short ID, it uses a `SqidsGen` helper and a unique set of numbers from `CountersArr()`.

```go
// CreateShortenedURL creates a new shortened URL from a long URL.
// It generates a short URL, stores it in the database, and returns the short URL.
func (s *URLServiceImpl) CreateShortenedURL(longURL string) (string, error) {
	// Get a unique array of numbers for ID generation
	shortURL := s.SqidsGen.Generate(s.CountersArr())

	// Store the new short URL and the original long URL
	if err := s.DBURLs.Set(shortURL, longURL); err != nil {
		if _, ok := err.(*types.BadRequestError); ok {
			return "", types.NewAppError("Bad request", "Invalid input data", http.StatusBadRequest, err)
		}
		return "", types.NewAppError("Failed to set URL", "Internal server error", http.StatusInternalServerError, err)
	}
	slog.Info("Shortened URL created", "shortURL", shortURL, "longURL", longURL)

	return shortURL, nil
}
```

The `CountersArr` function is key to uniqueness. It combines a local, incrementing counter with a database-backed counter (or a large random number if the DB is unavailable) to ensure the input to the `sqids` algorithm is always unique.

### 4.4. API Endpoints & Handlers (`routes/routes.go`, `handlers/handlers.go`)

Routes are registered in `routes.go`, which maps endpoints to specific handler functions and applies middleware.

```go
// RegisterAPIRoutesWithMiddleware registers API routes for the URL shortening service with middlewares.
func RegisterAPIRoutesWithMiddleware(mux *http.ServeMux, service service.URLService) handlers.ShortenedURLHandler {
	shortenedURLHandler := handlers.NewShortenedURLHandler(service)

	// API route for creating a shortened URL
	// POST /v1/shorten
	mux.Handle("/"+types.APIVersion+"/shorten", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.CreateShortenedURL)))

	// API route for retrieving a long URL from a shortened URL
	// GET /v1/shorten/{shortURL}
	mux.Handle("/"+types.APIVersion+"/shorten/{shortURL}", middleware.DBReadyMiddleware(http.HandlerFunc(shortenedURLHandler.GetShortenedURL)))

	return shortenedURLHandler
}
```

The handlers in `handlers.go` are responsible for decoding requests, calling the service layer, and sending back responses.

```go
// CreateShortenedURL handles the creation of a new shortened URL.
func (h *ShortenedURLHandlerImpl) CreateShortenedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		HandleError(w, types.NewAppError("Method Not Allowed", "Only POST method is allowed", http.StatusMethodNotAllowed, nil))
		return
	}

	// Decode the JSON payload from the request
	payload, err := types.DecodePayload(r)
	if err != nil {
		HandleError(w, types.NewAppError("Failed to decode payload", "Invalid request payload", http.StatusBadRequest, err))
		return
	}
    // ... (validation) ...

	// Call the service to perform the business logic
	shortURL, err := h.Service.CreateShortenedURL(payload.LongURL)
	if err != nil {
		HandleError(w, err)
		return
	}

	// Send a successful JSON response
	JSONResponse(w, http.StatusCreated, map[string]string{
		"shortURL": "/" + shortURL,
	})
}
```

### 4.5. Error Handling (`types/errors.go`, `handlers/handlers.go`)

The application uses a centralized error handling strategy. A generic `AppError` struct is used to wrap underlying errors and add important context, such as an HTTP status code and user-friendly messages.

```go
// AppError is a generic error type for the application.
type AppError struct {
	Underlying      error `json:"-"`
	HTTPStatus      int    `json:"-"`
	Message         string `json:"message"`
	InternalMessage string `json:"-"`
}
```

Factory functions like `NewDBError` and `NewConfigError` are used to create specific kinds of `AppError`s.

A helper function, `HandleError`, in the `handlers` package ensures that all errors are logged consistently and that a proper JSON error response is sent to the client.

```go
// HandleError is a utility function to handle errors in HTTP handlers.
func HandleError(w http.ResponseWriter, err error) {
	var appErr *types.AppError
	if errors.As(err, &appErr) {
		// This is our custom error type
		slog.Error("Handle Error", "Error", appErr) // Log the detailed error

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTPStatus)
		json.NewEncoder(w).Encode(appErr)
		return
	}

	// For any other error, return a generic 500.
	slog.Error("Handle Error", "An unexpected error occurred", err)
	http.Error(w, `{"message":"An internal server error occurred."}`, http.StatusInternalServerError)
}
```
