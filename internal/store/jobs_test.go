package store

import (
	"context"
	"errors"
	"testing"
)

func TestEnqueueAndClaimJob(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, err := s.EnqueueJob(ctx, "tag_item", map[string]int64{"item_id": 1})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	job, err := s.ClaimNextJob(ctx, "tag_item")
	if err != nil {
		t.Fatalf("ClaimNextJob: %v", err)
	}
	if job.ID != id {
		t.Errorf("claimed job id = %d, want %d", job.ID, id)
	}
	if job.Status != "pending" || job.Attempts != 0 {
		t.Errorf("unexpected job state: %+v", job)
	}
}

func TestClaimNextJobEmptyQueue(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ClaimNextJob(context.Background(), "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob on empty queue = %v, want ErrNotFound", err)
	}
}

func TestClaimNextJobIgnoresOtherTypes(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.EnqueueJob(ctx, "some_other_type", map[string]int64{}); err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	if _, err := s.ClaimNextJob(ctx, "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob(tag_item) with only other-typed jobs queued = %v, want ErrNotFound", err)
	}
}

func TestCompleteJobRemovesItFromQueue(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, _ := s.EnqueueJob(ctx, "tag_item", map[string]int64{"item_id": 1})
	if err := s.CompleteJob(ctx, id); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	if _, err := s.ClaimNextJob(ctx, "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob after completion = %v, want ErrNotFound", err)
	}
}

func TestFailJobReschedulesBelowMaxAttempts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, _ := s.EnqueueJob(ctx, "tag_item", map[string]int64{"item_id": 1})

	if err := s.FailJob(ctx, id, 0, "transient error"); err != nil {
		t.Fatalf("FailJob: %v", err)
	}

	// Rescheduled with backoff, so it should NOT be immediately claimable.
	if _, err := s.ClaimNextJob(ctx, "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob right after a reschedule = %v, want ErrNotFound (should be backed off)", err)
	}
}

func TestFailJobPermanentlyFailsAtMaxAttempts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, _ := s.EnqueueJob(ctx, "tag_item", map[string]int64{"item_id": 1})

	// attempts starts at maxJobAttempts-1, so this failure should be terminal.
	if err := s.FailJob(ctx, id, maxJobAttempts-1, "still broken"); err != nil {
		t.Fatalf("FailJob: %v", err)
	}

	var status string
	var attempts int
	row := s.db.QueryRowContext(ctx, `SELECT status, attempts FROM jobs WHERE id = ?`, id)
	if err := row.Scan(&status, &attempts); err != nil {
		t.Fatalf("query job: %v", err)
	}
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if attempts != maxJobAttempts {
		t.Errorf("attempts = %d, want %d", attempts, maxJobAttempts)
	}

	if _, err := s.ClaimNextJob(ctx, "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob for a permanently-failed job = %v, want ErrNotFound", err)
	}
}

func TestFailJobPermanently(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, _ := s.EnqueueJob(ctx, "tag_item", map[string]int64{"item_id": 1})
	if err := s.FailJobPermanently(ctx, id, "budget exceeded"); err != nil {
		t.Fatalf("FailJobPermanently: %v", err)
	}

	if _, err := s.ClaimNextJob(ctx, "tag_item"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob for a permanently-failed job = %v, want ErrNotFound", err)
	}
}
