package store

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

type User struct {
	ID          int64
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

var ErrNotFound = errors.New("not found")
var ErrEmailTaken = errors.New("email already taken")
var ErrBadCredentials = errors.New("invalid credentials")

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

func (db *DB) CreateUser(ctx context.Context, email, password, displayName string) (*User, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	var u User
	err = db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name) VALUES (?, ?, ?)
		 RETURNING id, email, display_name, created_at`,
		email, hash, displayName,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &u, nil
}

func (db *DB) GetUserByCredentials(ctx context.Context, email, password string) (*User, error) {
	var u User
	var hash string
	err := db.QueryRowContext(ctx,
		`SELECT id, email, display_name, created_at, password_hash FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBadCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	if !checkPassword(password, hash) {
		return nil, ErrBadCredentials
	}
	return &u, nil
}

func (db *DB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, email, display_name, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func checkPassword(password, encoded string) bool {
	parts := strings.SplitN(encoded, "$", 3)
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(hash, expected) == 1
}
