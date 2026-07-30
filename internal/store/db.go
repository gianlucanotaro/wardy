// Package store owns all SQL access. Handlers never write SQL; store never
// imports http.
package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/gianlucanotaro/wardy/migrations"
)

// Store wraps the database connection. All queries live in methods on this
// type, grouped by resource across the package's files (users.go,
// sessions.go, ...).
type Store struct {
	db *sql.DB
}

// Open opens the SQLite database at path, applies pragmas, runs any pending
// migrations, and returns a ready-to-use Store.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout=5000&_pragma=foreign_keys=on&_pragma=journal_mode=WAL", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite allows only one writer at a time. A single connection avoids
	// SQLITE_BUSY entirely rather than tuning around it — plenty for a
	// two-user household app.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrations.Run(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
