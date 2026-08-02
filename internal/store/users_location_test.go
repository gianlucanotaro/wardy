package store

import (
	"context"
	"testing"
)

func TestUpdateUserLocation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	userID, err := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	user, err := s.UserByID(ctx, userID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if user.HomeLat.Valid || user.HomeLon.Valid {
		t.Errorf("new user should have no home location set, got %+v", user)
	}

	if err := s.UpdateUserLocation(ctx, userID, 47.3769, 8.5417); err != nil {
		t.Fatalf("UpdateUserLocation: %v", err)
	}

	user, err = s.UserByID(ctx, userID)
	if err != nil {
		t.Fatalf("UserByID after update: %v", err)
	}
	if !user.HomeLat.Valid || user.HomeLat.Float64 != 47.3769 {
		t.Errorf("HomeLat = %+v, want 47.3769", user.HomeLat)
	}
	if !user.HomeLon.Valid || user.HomeLon.Float64 != 8.5417 {
		t.Errorf("HomeLon = %+v, want 8.5417", user.HomeLon)
	}
}
