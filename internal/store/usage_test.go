package store

import (
	"context"
	"testing"
)

func TestLogUsageAndMonthlySpend(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.LogUsage(ctx, "claude-haiku-4-5-20251001", 500, 100, 0.001); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}
	if err := s.LogUsage(ctx, "claude-haiku-4-5-20251001", 500, 100, 0.002); err != nil {
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
