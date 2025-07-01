package database

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pizza-nz/url-shortener/types"
)

var (
	// dbReady indicates whether the database is connected and ready to accept queries.
	dbReady bool = false
)

// Database is an interface for URL storage.
// It defines methods for getting and setting URL data.
type Database interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// CounterDatabase is an interface for a counter.
// It defines a method for getting and incrementing a counter value.
type CounterDatabase interface {
	GetAndIncreament() (uint64, error)
}

// DatabaseURLPGImpl is a PostgreSQL implementation of the Database interface.
// It uses a pgxpool for connection pooling.
type DatabaseURLPGImpl struct {
	URLs *pgxpool.Pool
}

// DatabaseURLMapImpl is a thread-safe in-memory implementation of the Database interface.
// It uses a map for storing URLs with their corresponding short keys.
type DatabaseURLMapImpl struct {
	lock sync.RWMutex
	URLs map[string]string
}

// StartNewDatabase initializes and returns a database instance based on the connection string.
// It supports in-memory and PostgreSQL databases.
func StartNewDatabase(conn string) (Database, error) {
	switch {
	case conn == "":
		return mapDB(), nil
	case conn[:4] == "post":
		err := pingDB(conn)
		if err != nil {
			return nil, err
		}

		db, err := postgresDB(conn)
		if err != nil {
			return nil, err
		}
		return db, nil
	default:
		return nil, nil
	}
}

// pingDB checks the connection to the database.
// It sets the dbReady flag to true if the connection is successful.
func pingDB(conn string) error {
	ctx := context.Background()

	pgx, err := pgx.Connect(ctx, conn)
	if err != nil {
		return types.NewDBError("pingDB failed to pgx connect to DB", err)
	}
	if err := pgx.Ping(ctx); err != nil {
		return types.NewDBError("pingDB failed to ping to DB", err)
	}

	dbReady = true

	return nil
}

// IsDBReady returns the status of the database connection.
func IsDBReady() bool {
	return dbReady
}

// mapDB creates a new instance of DatabaseURLMapImpl.
// It initializes the internal map to ensure it is ready for use.
func mapDB() Database {
	return &DatabaseURLMapImpl{
		URLs: make(map[string]string),
	}
}

// Get retrieves the long URL associated with the given short key from the in-memory map.
// It returns a NotFoundError if the key does not exist.
func (m *DatabaseURLMapImpl) Get(key string) (string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	value, exists := m.URLs[key]
	if !exists {
		return "", types.NewNotFoundError(key)
	}
	return value, nil
}

// Set adds a new key-value pair to the in-memory map.
// It returns a BadRequestError if the key or value is empty, or if the key already exists.
func (m *DatabaseURLMapImpl) Set(key, value string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	details := []types.Details{}
	if key == "" {
		details = append(details, types.Details{Field: "key", Issue: "cannot be empty"})
	}
	if value == "" {
		details = append(details, types.Details{Field: "value", Issue: "cannot be empty"})
	}
	if len(details) > 0 {
		return types.NewBadRequestError(details)
	}
	if _, exists := m.URLs[key]; exists {
		details = append(details, types.Details{Field: "key", Issue: fmt.Sprintf("key '%s' already exists", key)})
		return types.NewBadRequestError(details)
	}

	m.URLs[key] = value
	slog.Info("URL added to map", "key", key, "value", value)

	return nil
}

// Get retrieves the long URL associated with the given short key from the PostgreSQL database.
// It returns a NotFoundError if the key does not exist.
func (db *DatabaseURLPGImpl) Get(key string) (string, error) {
	var longURL string
	err := db.URLs.QueryRow(context.Background(), "select long_url from table_urls where short_url=$1", key).Scan(&longURL)
	switch err {
	case nil:
		return longURL, nil
	case pgx.ErrNoRows:
		return "", types.NewNotFoundError(key)
	default:
		return "", types.NewDBError("Internal Server Error", nil)
	}
}

// Set adds a new key-value pair to the PostgreSQL database.
// It uses a transaction to ensure atomicity.
func (db *DatabaseURLPGImpl) Set(key, value string) error {
	tx, err := db.URLs.Begin(context.Background())
	if err != nil {
		return types.NewDBError("Postgres DB failed to begin a transcation", err)
	}
	_, err = tx.Exec(context.Background(), `insert into table_urls(short_url, long_url) values ($1, $2) 
	on conflict (short_url) do update set short_url=excluded.short_url`,
		key,
		value)
	if err != nil {
		tx.Rollback(context.Background())
		return types.NewDBError("Postgres DB failed to set new row", err)
	}

	return tx.Commit(context.Background())
}

// GetAndIncreament retrieves the current counter value from the database and increments it.
// It uses a transaction to ensure atomicity.
func (db *DatabaseURLPGImpl) GetAndIncreament() (uint64, error) {
	tx, err := db.URLs.Begin(context.Background())
	if err != nil {
		return 0, types.NewDBError("Postgres DB failed to begin a transcation", err)
	}
	createdAt := time.Now()
	_, err = tx.Exec(context.Background(), `insert into table_counter (created_at) values ($1)`, createdAt)
	if err != nil {
		tx.Rollback(context.Background())
		return 0, types.NewDBError("Counter DB failed to set new row", err)
	}
	var counter uint64
	_ = tx.QueryRow(context.Background(), `SELECT count(*) from table_counter`).Scan(&counter)

	return counter, tx.Commit(context.Background())
}

// postgresDB creates a new PostgreSQL database instance.
// It runs migrations and sets up a connection pool.
func postgresDB(conn string) (Database, error) {
	if conn == "" {
		return nil, types.NewDBError("PGConnnectionString not set, were you meant to use NewDatabaseURLMapImpl?", nil)
	}

	if err := Migration(conn); err != nil {
		return nil, types.NewDBError("poolconfig failed to migrate", err)
	}

	poolConfig, err := pgxpool.ParseConfig(conn)
	if err != nil {
		return nil, types.NewDBError("poolconfig failed to parse", err)
	}

	db, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, types.NewDBError("poolconfig failed to create new pool", err)
	}

	if err = db.Ping(context.Background()); err != nil {
		return nil, types.NewDBError("DB pool failed to ping PG", err)
	}

	return &DatabaseURLPGImpl{
		URLs: db,
	}, nil
}