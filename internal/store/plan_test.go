package store

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestPlanDaysInRangeCarriesWears(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	planned, _ := s.CreateItem(ctx, userID, "p.jpg", "pt.jpg")
	worn, _ := s.CreateItem(ctx, userID, "w.jpg", "wt.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{planned})
	if err := s.UpsertPlanDay(ctx, userID, "2026-09-14", &lookID); err != nil {
		t.Fatalf("UpsertPlanDay: %v", err)
	}
	if _, err := s.LogWear(ctx, userID, nil, []int64{worn}, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}
	// The day after the range: must not show up.
	if _, err := s.LogWear(ctx, userID, nil, []int64{worn}, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	days, err := s.PlanDaysInRange(ctx, userID, "2026-09-14", "2026-09-20")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days[0].Wears) != 1 || len(days[0].Wears[0].Items) != 1 || days[0].Wears[0].Items[0].ID != worn {
		t.Errorf("days[0].Wears = %+v, want one wear of item %d", days[0].Wears, worn)
	}
	for _, d := range days[1:] {
		if d.Wears == nil || len(d.Wears) != 0 {
			t.Errorf("%s: Wears = %#v, want an empty, non-nil list", d.Date, d.Wears)
		}
	}
}

func TestPlanDaysInRangeWearsAreScopedToUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemB, _ := s.CreateItem(ctx, userB, "b.jpg", "bt.jpg")
	if _, err := s.LogWear(ctx, userB, nil, []int64{itemB}, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	days, err := s.PlanDaysInRange(ctx, userA, "2026-09-14", "2026-09-14")
	if err != nil {
		t.Fatalf("PlanDaysInRange: %v", err)
	}
	if len(days) != 1 || len(days[0].Wears) != 0 {
		t.Errorf("user A's plan shows user B's wears: %+v", days)
	}
}
