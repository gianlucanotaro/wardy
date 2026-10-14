package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSetPassword(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	alice, err := s.CreateUser(ctx, "alice", "old-hash", "Alice")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := s.CreateUser(ctx, "bob", "bob-hash", "Bob")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	expires := time.Now().Add(time.Hour)
	for token, userID := range map[string]int64{"a1": alice, "a2": alice, "b1": bob} {
		if err := s.CreateSession(ctx, token, userID, expires); err != nil {
			t.Fatalf("create session %s: %v", token, err)
		}
	}

	if err := s.SetPassword(ctx, "alice", "new-hash"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	u, err := s.UserByEmail(ctx, "alice")
	if err != nil {
		t.Fatalf("load alice: %v", err)
	}
	if u.PasswordHash != "new-hash" {
		t.Errorf("alice hash = %q, want new-hash", u.PasswordHash)
	}
	for _, token := range []string{"a1", "a2"} {
		if _, err := s.SessionUser(ctx, token); !errors.Is(err, ErrNotFound) {
			t.Errorf("alice session %s still valid (err=%v), want signed out", token, err)
		}
	}

	// Other users are untouched.
	if u, err := s.UserByEmail(ctx, "bob"); err != nil || u.PasswordHash != "bob-hash" {
		t.Errorf("bob changed: hash=%q err=%v", u.PasswordHash, err)
	}
	if _, err := s.SessionUser(ctx, "b1"); err != nil {
		t.Errorf("bob's session was revoked: %v", err)
	}

	if err := s.SetPassword(ctx, "nobody", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
}
