package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

// DB wraps pgxpool.Pool so the rest of the application
// imports only this package rather than pgx directly.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect establishes a connection pool to PostgreSQL.
// Call once in main.go and pass the resulting *DB to repositories.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: failed to parse config: %w", err)
	}

	// Pool sizing — tune based on your server's resources
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: failed to create pool: %w", err)
	}

	// Verify the connection is alive before returning
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close releases all connections in the pool.
// Call in main.go during graceful shutdown.
func (db *DB) Close() {
	db.Pool.Close()
}

// Ping checks that the database is still reachable.
// Used by a health check endpoint.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// RunMigrations applies all pending migrations.
// Call this in main.go after Connect() and before starting the HTTP server.
func RunMigrations(databaseURL, migrationsPath string) error {
    m, err := migrate.New(
        "file://"+migrationsPath,
        databaseURL,
    )
    if err != nil {
        return fmt.Errorf("database: failed to load migrations: %w", err)
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("database: migration failed: %w", err)
    }

    return nil
}