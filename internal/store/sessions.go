package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateSession records a new opaque session token for userID, expiring at
// expiresAt.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// SessionUser returns the user for a valid, unexpired session token.
// Expired or unknown tokens both return ErrNotFound — from the caller's
// perspective there's no meaningful difference.
func (s *Store) SessionUser(ctx context.Context, token string) (*User, error) {
	var u User
	var expiresAt string

	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.password_hash, u.display_name, u.home_lat, u.home_lon, u.created_at, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, token,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.HomeLat, &u.HomeLon, &u.CreatedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}
	if time.Now().After(exp) {
		return nil, ErrNotFound
	}

	return &u, nil
}

// DeleteSession removes a session token, if present. Deleting an
// already-gone token is not an error.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
