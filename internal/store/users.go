package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a query finds no matching row.
var ErrNotFound = errors.New("not found")

// User is a registered account. There is no public signup — rows are
// created only via the create-user CLI command.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    string
}

// CreateUser inserts a new user and returns its id. email must be unique;
// callers should treat a failure here as "email already exists" unless the
// underlying error says otherwise.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, displayName string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, display_name) VALUES (?, ?, ?)`,
		email, passwordHash, displayName)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted id: %w", err)
	}

	return id, nil
}

// UserByEmail looks up a user by email, returning ErrNotFound if none
// exists.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, created_at FROM users WHERE email = ?`,
		email))
}

// UserByID looks up a user by id, returning ErrNotFound if none exists.
func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, created_at FROM users WHERE id = ?`,
		id))
}

func (s *Store) scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}
