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
	HomeLat      NullFloat64
	HomeLon      NullFloat64
	// UseInspo: whether outfit suggestions use the style profile built
	// from the user's inspiration board (#49).
	UseInspo  bool
	CreatedAt string
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

const userColumns = `SELECT id, email, password_hash, display_name, home_lat, home_lon, use_inspo, created_at`

// UserByEmail looks up a user by email, returning ErrNotFound if none
// exists.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		userColumns+` FROM users WHERE email = ?`, email))
}

// UserByID looks up a user by id, returning ErrNotFound if none exists.
func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		userColumns+` FROM users WHERE id = ?`, id))
}

// UpdateUserLocation sets a user's home location, used to fetch weather
// (internal/weather) without ever demanding browser geolocation.
func (s *Store) UpdateUserLocation(ctx context.Context, userID int64, lat, lon float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET home_lat = ?, home_lon = ? WHERE id = ?`, lat, lon, userID)
	if err != nil {
		return fmt.Errorf("update user location: %w", err)
	}
	return nil
}

// SetUseInspo turns the style profile on or off for userID's outfit
// suggestions.
func (s *Store) SetUseInspo(ctx context.Context, userID int64, use bool) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET use_inspo = ? WHERE id = ?`, use, userID); err != nil {
		return fmt.Errorf("update use_inspo: %w", err)
	}
	return nil
}

// StyleSummary returns userID's cached style summary and the profile key
// it was written for — both empty if none was written yet.
func (s *Store) StyleSummary(ctx context.Context, userID int64) (summary, key string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT style_summary, style_summary_key FROM users WHERE id = ?`, userID).Scan(&summary, &key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("query style summary: %w", err)
	}
	return summary, key, nil
}

// SetStyleSummary caches the style summary written for the profile
// identified by key.
func (s *Store) SetStyleSummary(ctx context.Context, userID int64, summary, key string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET style_summary = ?, style_summary_key = ? WHERE id = ?`, summary, key, userID); err != nil {
		return fmt.Errorf("update style summary: %w", err)
	}
	return nil
}

func (s *Store) scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.HomeLat, &u.HomeLon, &u.UseInspo, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// SetPassword replaces the password hash for the user with this login and
// deletes all of their sessions, so a changed password signs out every
// device. Returns ErrNotFound if no such user exists.
func (s *Store) SetPassword(ctx context.Context, email, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("look up user: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
