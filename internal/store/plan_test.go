package store

import (
	"context"
	"errors"
	"testing"
)

func TestUpsertPlanDayAssignsAndClears(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{itemID})

	if err := s.UpsertPlanDay(ctx, userID, "2026-09-14", &lookID); err != nil {
		t.Fatalf("UpsertPlanDay: %v", err)
	}

	days, err := s.PlanDaysInRange(ctx, userID, "2026-09-14", "2026-09-14")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 1 || days[0].Look == nil || days[0].Look.ID != lookID {
		t.Fatalf("unexpected days: %+v", days)
	}

	// Re-assigning the same date should overwrite, not duplicate.
	if err := s.UpsertPlanDay(ctx, userID, "2026-09-14", nil); err != nil {
		t.Fatalf("UpsertPlanDay(clear): %v", err)
	}
	days, err = s.PlanDaysInRange(ctx, userID, "2026-09-14", "2026-09-14")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 1 || days[0].Look != nil {
		t.Fatalf("expected the day to be cleared, got: %+v", days)
	}
}

func TestUpsertPlanDayRejectsLookFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemID, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userA, "Look", "", []int64{itemID})

	if err := s.UpsertPlanDay(ctx, userB, "2026-09-14", &lookID); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpsertPlanDay with another user's look = %v, want ErrNotFound", err)
	}

	days, err := s.PlanDaysInRange(ctx, userB, "2026-09-14", "2026-09-14")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 1 || days[0].Look != nil {
		t.Errorf("rejected upsert should leave the day unplanned, got: %+v", days)
	}
}

func TestPlanDaysInRangeFillsUnplannedDates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	days, err := s.PlanDaysInRange(ctx, userID, "2026-09-14", "2026-09-20")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 7 {
		t.Fatalf("got %d days, want 7", len(days))
	}
	if days[0].Date != "2026-09-14" || days[6].Date != "2026-09-20" {
		t.Errorf("unexpected date range: %+v", days)
	}
	for _, d := range days {
		if d.Look != nil {
			t.Errorf("expected every day unplanned, got a look on %s", d.Date)
		}
	}
}

func TestPlanDaysInRangeIsScopedToUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemID, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userA, "Look", "", []int64{itemID})
	if err := s.UpsertPlanDay(ctx, userA, "2026-09-14", &lookID); err != nil {
		t.Fatalf("UpsertPlanDay: %v", err)
	}

	days, err := s.PlanDaysInRange(ctx, userB, "2026-09-14", "2026-09-14")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 1 || days[0].Look != nil {
		t.Errorf("user B should not see user A's plan, got: %+v", days)
	}
}
