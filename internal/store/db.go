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

// Open opens the SQLite database at path, applies pragmas, runs any pending
// migrations, and returns a ready-to-use connection pool.
func Open(ctx context.Context, path string) (*sql.DB, error) {
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

	return db, nil
}
