package store

import (
	"context"
	"testing"
)

func TestLogUsageAndMonthlySpend(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.LogUsage(ctx, 1, "claude-haiku-4-5-20251001", 500, 100, 0.001); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}
	if err := s.LogUsage(ctx, 1, "claude-haiku-4-5-20251001", 500, 100, 0.002); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}

	total, err := s.MonthlySpend(ctx)
	if err != nil {
		t.Fatalf("MonthlySpend: %v", err)
	}
	if want := 0.003; total < want-0.0001 || total > want+0.0001 {
		t.Errorf("MonthlySpend = %v, want ~%v", total, want)
	}
}

func TestMonthlySpendZeroWhenNoUsage(t *testing.T) {
	total, err := newTestStore(t).MonthlySpend(context.Background())
	if err != nil {
		t.Fatalf("MonthlySpend: %v", err)
	}
	if total != 0 {
		t.Errorf("MonthlySpend on empty usage table = %v, want 0", total)
	}
}

func TestDailyUsageCount(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const model = "claude-sonnet-5"
	for i := 0; i < 3; i++ {
		if err := s.LogUsage(ctx, 1, model, 100, 50, 0.001); err != nil {
			t.Fatalf("LogUsage: %v", err)
		}
	}
	// A different user and a different model must not count toward user 1's
	// cap.
	if err := s.LogUsage(ctx, 2, model, 100, 50, 0.001); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}
	if err := s.LogUsage(ctx, 1, "claude-haiku-4-5-20251001", 100, 50, 0.001); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}

	count, err := s.DailyUsageCount(ctx, 1, model)
	if err != nil {
		t.Fatalf("DailyUsageCount: %v", err)
	}
	if count != 3 {
		t.Errorf("DailyUsageCount = %d, want 3", count)
	}
}

func TestDailyUsageCountZeroWhenNoUsage(t *testing.T) {
	count, err := newTestStore(t).DailyUsageCount(context.Background(), 1, "claude-sonnet-5")
	if err != nil {
		t.Fatalf("DailyUsageCount: %v", err)
	}
	if count != 0 {
		t.Errorf("DailyUsageCount on empty usage table = %d, want 0", count)
	}
}
