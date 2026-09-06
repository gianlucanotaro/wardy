package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
)

// fakeProvider lets each test script exactly what TagItem returns, and
// records whether it was called at all — the budget-guard test needs to
// assert the model was never even invoked.
type fakeProvider struct {
	result ai.TagResult
	usage  ai.Usage
	err    error
	called bool
}

func (f *fakeProvider) TagItem(_ context.Context, _ []byte) (ai.TagResult, ai.Usage, error) {
	f.called = true
	return f.result, f.usage, f.err
}

func (f *fakeProvider) SuggestOutfits(_ context.Context, _ []ai.SuggestionCandidate, _, _, _ string) ([]ai.AISuggestion, ai.Usage, error) {
	return nil, ai.Usage{}, errors.New("SuggestOutfits not used by jobs package tests")
}

func validFakeResult() ai.TagResult {
	return ai.TagResult{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3,
		Seasons: []string{"summer"}, Occasions: []string{"casual"},
	}
}

// newTestFixture creates a user, an item with a real thumbnail file on
// disk (the worker reads it), and returns everything needed to enqueue and
// process a tag_item job.
func newTestFixture(t *testing.T) (s *store.Store, photosDir string, userID, itemID int64) {
	t.Helper()
	ctx := context.Background()

	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	photosDir = t.TempDir()

	userID, err = s.CreateUser(ctx, "me@example.com", "hash", "Me")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(photosDir, "1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	thumbRel := filepath.Join("1", "item.jpg")
	if err := os.WriteFile(filepath.Join(photosDir, thumbRel), []byte("fake-thumbnail-bytes"), 0o644); err != nil {
		t.Fatalf("write fake thumb: %v", err)
	}

	itemID, err = s.CreateItem(ctx, userID, thumbRel, thumbRel)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	return s, photosDir, userID, itemID
}

func enqueueTagJob(t *testing.T, s *store.Store, userID, itemID int64) int64 {
	t.Helper()
	id, err := s.EnqueueJob(context.Background(), TagItemType, TagItemPayload{ItemID: itemID, UserID: userID})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	return id
}

func TestProcessOneSuccess(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	provider := &fakeProvider{result: validFakeResult(), usage: ai.Usage{Model: "fake", InputTokens: 100, OutputTokens: 20, EstCostUSD: 0.001}}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	if !provider.called {
		t.Fatal("provider was never called")
	}

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "done" {
		t.Errorf("job status = %q, want done", job.Status)
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if item.Category != "top" || !item.AITaggedAt.Valid {
		t.Errorf("tags not applied: %+v", item)
	}

	spend, err := s.MonthlySpend(ctx)
	if err != nil {
		t.Fatalf("MonthlySpend: %v", err)
	}
	if spend <= 0 {
		t.Error("usage should have been logged")
	}
}

func TestProcessOneRetriesOnTransientError(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	provider := &fakeProvider{err: errors.New("connection reset")}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "pending" {
		t.Errorf("job status = %q, want still pending (rescheduled)", job.Status)
	}
	if job.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", job.Attempts)
	}
	if job.LastError == "" {
		t.Error("last_error should be recorded")
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if item.AITaggedAt.Valid {
		t.Errorf("item should remain untagged after a failed attempt, got ai_tagged_at set")
	}
}

func TestProcessOneRetriesOnOutOfVocabResponse(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	badResult := validFakeResult()
	badResult.Category = "sock" // not a real category

	provider := &fakeProvider{result: badResult, usage: ai.Usage{Model: "fake"}}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "pending" || job.Attempts != 1 {
		t.Errorf("out-of-vocab response should be retried, got status=%q attempts=%d", job.Status, job.Attempts)
	}

	item, _ := s.ItemByID(ctx, userID, itemID)
	if item.AITaggedAt.Valid {
		t.Errorf("invalid tags must never be written, got ai_tagged_at set")
	}
	if item.Category != "top" {
		t.Errorf("item should still have its default tags, got category %q", item.Category)
	}
}

func TestProcessOneSkipsWhenAIDisabled(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	provider := &fakeProvider{err: ai.ErrDisabled}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "done" {
		t.Errorf("a disabled provider should complete the job as a no-op, got status %q", job.Status)
	}

	item, _ := s.ItemByID(ctx, userID, itemID)
	if item.AITaggedAt.Valid {
		t.Errorf("item should remain untagged when AI is disabled, got ai_tagged_at set")
	}
}

func TestProcessOneNeverOverwritesUserEditedItem(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)

	// A manual edit lands before the AI job runs.
	if err := s.UpdateItemTags(ctx, userID, itemID, store.ItemTags{
		Category: "bottom", Subcategory: "jeans", ColorPrimary: "navy",
		Pattern: "solid", Material: "denim", Fit: "slim",
		Formality: 2, Warmth: 3, Seasons: []string{"autumn"}, Occasions: []string{"casual"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	jobID := enqueueTagJob(t, s, userID, itemID)
	provider := &fakeProvider{result: validFakeResult(), usage: ai.Usage{Model: "fake"}}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "done" {
		t.Errorf("job status = %q, want done", job.Status)
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if item.Category != "bottom" {
		t.Errorf("user's manual tags were overwritten by the AI run: %+v", item)
	}
	if item.AITaggedAt.Valid {
		t.Error("ai_tagged_at should not be set when the AI update was skipped")
	}
}

func TestProcessOneSkipsWhenItemDeleted(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	if _, _, err := s.DeleteItem(ctx, userID, itemID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	provider := &fakeProvider{result: validFakeResult(), usage: ai.Usage{Model: "fake"}}
	w := NewWorker(s, provider, photosDir, 0)
	w.ProcessOne(ctx)

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "done" {
		t.Errorf("a job for a deleted item should complete as a no-op, got status %q", job.Status)
	}
	if provider.called {
		t.Error("provider should not be called for a deleted item")
	}
}

func TestProcessOneSkipsCallWhenBudgetExceeded(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	jobID := enqueueTagJob(t, s, userID, itemID)

	// Already over a tiny budget before the worker even looks at the job.
	if err := s.LogUsage(ctx, userID, "claude-haiku-4-5-20251001", 1000, 1000, 1.00); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}

	provider := &fakeProvider{result: validFakeResult(), usage: ai.Usage{Model: "fake"}}
	w := NewWorker(s, provider, photosDir, 0.01) // budget already exceeded
	w.ProcessOne(ctx)

	if provider.called {
		t.Error("provider must not be called once the monthly budget is exceeded")
	}

	job, err := s.JobByID(ctx, jobID)
	if err != nil {
		t.Fatalf("JobByID: %v", err)
	}
	if job.Status != "failed" {
		t.Errorf("job status = %q, want failed (permanently, no retry)", job.Status)
	}

	item, _ := s.ItemByID(ctx, userID, itemID)
	if item.AITaggedAt.Valid {
		t.Errorf("item should remain untagged, got ai_tagged_at set")
	}
}

func TestProcessOneNoJobsAvailable(t *testing.T) {
	s, photosDir, _, _ := newTestFixture(t)
	provider := &fakeProvider{}
	w := NewWorker(s, provider, photosDir, 0)

	// Should be a no-op, not a panic or error, when the queue is empty.
	w.ProcessOne(context.Background())

	if provider.called {
		t.Error("provider should not be called when there's no job to process")
	}
}
