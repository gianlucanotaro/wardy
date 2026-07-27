package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

const sessionDuration = 30 * 24 * time.Hour

func (db *DB) CreateSession(ctx context.Context, userID int64) (*Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	s := &Session{
		Token:     hex.EncodeToString(b),
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		s.Token, s.UserID, s.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return s, nil
}

func (db *DB) GetSession(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := db.QueryRowContext(ctx,
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > CURRENT_TIMESTAMP`,
		token,
	).Scan(&s.Token, &s.UserID, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &s, nil
}

func (db *DB) DeleteSession(ctx context.Context, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (db *DB) PruneExpiredSessions(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}
