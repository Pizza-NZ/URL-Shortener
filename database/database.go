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
	dbReady bool = false
)

type Database interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

type CounterDatabase interface {
	GetAndIncreament() (uint64, error)
}

type DatabaseURLPGImpl struct {
	URLs *pgxpool.Pool
}

// DatabaseURLMapImpl is a thread-safe map for storing URLs with their corresponding short keys.
type DatabaseURLMapImpl struct {
	lock sync.RWMutex
	URLs map[string]string
}

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

func IsDBReady() bool {
	return dbReady
}

// NewDatabaseURLMapImpl creates a new instance of DatabaseURLMapImpl.
// It initializes the internal map to ensure it is ready for use.
func mapDB() Database {
	return &DatabaseURLMapImpl{
		URLs: make(map[string]string),
	}
}

// Get retrieves the value associated with the given key from the URLMap.
// If the key does not exist, it returns a NotFoundError.
// It uses a read lock to ensure thread safety during the retrieval.
// The returned value is the long URL associated with the short key.
func (m *DatabaseURLMapImpl) Get(key string) (string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	value, exists := m.URLs[key]
	if !exists {
		return "", types.NewNotFoundError(key)
	}
	return value, nil
}

// Set adds a new key-value pair to the URLMap.
// It checks if the key and value are not empty and if the key does not already exist.
// If any of these conditions are not met, it returns a BadRequestError with appropriate details.
// It uses a write lock to ensure thread safety during the addition.
// If successful, it logs the addition of the URL to the map.
// If the key already exists, it returns a BadRequestError indicating that the key is already
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
