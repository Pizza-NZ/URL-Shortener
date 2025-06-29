# Go URL Shortener

A robust and high-performance URL shortening service built with Go. This project provides a simple API to create short, unique identifiers for long URLs and redirects users to the original URL when the short link is visited. The service can be run in two modes: using an in-memory database for quick testing and development, or with a persistent PostgreSQL database for production use.

## Features

- **Dual Database Support**: Run with either a transient in-memory map or a persistent PostgreSQL database.
- **RESTful API**: Simple and clean API for creating and retrieving short URLs.
- **Unique ID Generation**: Leverages `sqids` to generate unique, short, non-sequential IDs.
- **Structured Logging**: Implements structured JSON logging with `slog` for better observability.
- **Request Tracing**: A middleware injects a unique `X-Request-ID` into every request for end-to-end traceability.
- **Graceful Shutdown**: The server gracefully shuts down, allowing in-flight requests to complete before exiting.
- **Containerized**: Fully containerized with a multi-stage `Dockerfile` and `docker-compose.yml` for a complete and secure production environment.
- **Database Migrations**: Includes a simple migration system to manage the database schema.

## Architecture

The project follows a clean architecture with a clear separation of concerns:

- **Handlers**: Responsible for parsing incoming HTTP requests, validating input, and calling the appropriate service methods.
- **Services**: Contain the core business logic of the application, such as creating and retrieving URLs.
- **Database**: An abstraction layer for data persistence, with implementations for both in-memory and PostgreSQL databases.
- **Middleware**: Provides common functionality like request tracing and database readiness checks.

## API Documentation

All API endpoints are prefixed with `/v1`.

### Create a Short URL

Creates a new short URL for a given long URL.

- **Endpoint**: `POST /v1/shorten`
- **Method**: `POST`
- **Request Body**:
  ```json
  {
    "LongURL": "https://www.google.com/search?q=golang+best+practices"
  }
  ```
- **Success Response (201 Created)**:
  ```json
  {
    "shortURL": "/jR"
  }
  ```
- **Error Response (400 Bad Request)**:
  ```json
  {
    "message": "Bad Request",
    "details": [
      {
        "field": "LongURL",
        "issue": "Long URL cannot be empty"
      }
    ]
  }
  ```

### Redirect to Long URL

Redirects the client to the original long URL associated with the short URL.

- **Endpoint**: `GET /v1/shorten/{shortURL}`
- **Method**: `GET`
- **Example**: `GET /v1/shorten/jR`
- **Success Response (301 Moved Permanently)**:
    - Redirects to the `LongURL` specified during creation.
- **Error Response (404 Not Found)**:
    - Returned if the `{shortURL}` does not exist in the database.
  ```json
  {
    "message": "Not Found"
  }
  ```

## Configuration

The application is configured using environment variables.

### Server Configuration

- `LISTENADDR`: The address for the server to listen on. (Default: `:1232`)
- `READTIMEOUT`: Read timeout in milliseconds. (Default: `10000`)
- `WRITETIMEOUT`: Write timeout in milliseconds. (Default: `10000`)
- `IDLETIMEOUT`: Idle timeout in milliseconds. (Default: `120000`)

### Database Configuration

- `DB_HOST`: The database host. (Default: `localhost`)
- `DB_PORT`: The database port. (Default: `5432`)
- `DB_NAME`: The name of the database. (Default: `url_shortener`)
- `DB_USER`: The database user. (Default: `user`)
- `DB_PASS`: The database password. (Default: `password`)

## Getting Started

Follow these instructions to get the project running on your local machine.

### Prerequisites

- Go v1.22 or later
- Docker and Docker Compose (for containerized approach)
- `make` (optional, for using the Makefile)

### Installation & Setup

1.  **Clone the repository**:
    ```bash
    git clone <your-repository-url>
    cd <repository-directory>
    ```

2.  **Run the application**:

    - **In-Memory Mode**: To run the application with an in-memory database, simply run the following command:
      ```bash
      go run ./cmd/main.go
      ```

    - **PostgreSQL Mode**: To run with a PostgreSQL database, first ensure you have a running PostgreSQL instance. Then, set the required database environment variables and run the application:
      ```bash
      export DB_HOST=localhost
      export DB_PORT=5432
      export DB_NAME=url_shortener
      export DB_USER=user
      export DB_PASS=password
      go run ./cmd/main.go
      ```

## Running with Docker

The easiest way to run the application with a PostgreSQL database is by using Docker Compose.

1.  **Create a `.env` file** with the following content:
    ```
    DB_USER=user
    DB_PASS=password
    DB_NAME=url_shortener
    LISTENADDR=:1232
    ```

2.  **Run Docker Compose**:
    ```bash
    docker-compose up -d
    ```

This will start the application and a PostgreSQL database. The service will be accessible on `http://localhost:1232`.

## Testing

To run the unit tests for the project, use the following command:

```bash
go test ./...
```

This will run all tests in the project and show the results.

## Makefile

The project includes a `Makefile` with the following commands:

- `make build`: Build the Go application binary.
- `make run`: Build and run the application.
- `make clean`: Remove the application binary.
- `make test`: Run the unit tests.

## Future Work

- **User Authentication**: Add user accounts so that users can manage their own links.
- **Link Analytics**: Track the number of clicks for each short URL.
- **Custom Short URLs**: Allow users to specify a custom short URL string.
- **Link Expiration**: Add an option for links to expire after a certain amount of time or number of clicks.