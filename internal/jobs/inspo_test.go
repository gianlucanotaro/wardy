package jobs

import (
	"context"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
)

func validInspoResult() ai.InspoTagResult {
	return ai.InspoTagResult{
		Styles: []string{"minimal"}, Colors: []string{"black"}, Pieces: []string{"jeans"},
		Fit: "relaxed", FormalityMin: 2, FormalityMax: 3,
	}
}

// newInspoFixture is newTestFixture's user and fake thumbnail as an inspo
// fit, returning its id.
func newInspoFixture(t *testing.T) (s *store.Store, photosDir string, userID, fitID int64) {
	t.Helper()
	s, photosDir, userID, itemID := newTestFixture(t)
	item, err := s.ItemByID(context.Background(), userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	fitID, err = s.CreateInspoFit(context.Background(), userID, item.PhotoPath, item.ThumbPath)
	if err != nil {
		t.Fatalf("CreateInspoFit: %v", err)
	}
	return s, photosDir, userID, fitID
}

func TestTagInspoAppliesTagsAndQueuesSummary(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, fitID := newInspoFixture(t)
	jobID, _ := s.EnqueueJob(ctx, TagInspoType, TagInspoPayload{FitID: fitID, UserID: userID})

	provider := &fakeProvider{inspoResult: validInspoResult(), usage: ai.Usage{Model: "fake"}}
	NewWorker(s, provider, photosDir, 0).ProcessOne(ctx)

	if job, _ := s.JobByID(ctx, jobID); job.Status != "done" {
		t.Fatalf("job status = %q (%s), want done", job.Status, job.LastError)
	}
	fit, _ := s.InspoFitByID(ctx, userID, fitID)
	if len(fit.Styles) != 1 || fit.Styles[0] != "minimal" || !fit.AITaggedAt.Valid {
		t.Errorf("fit after tagging = %+v", fit)
	}
	if pending, _ := s.PendingTagInspoIDs(ctx); pending[fitID] {
		t.Error("fit still pending after its job finished")
	}
	// The profile changed, so a (debounced, not yet due) summary job waits.
	if _, err := s.ClaimNextJob(ctx, SummarizeStyleType); err != store.ErrNotFound {
		t.Errorf("summary job due right away (%v), want it debounced", err)
	}
}

func TestTagInspoRejectsOutOfVocabulary(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, fitID := newInspoFixture(t)
	jobID, _ := s.EnqueueJob(ctx, TagInspoType, TagInspoPayload{FitID: fitID, UserID: userID})

	bad := validInspoResult()
	bad.Styles = []string{"cottagecore"}
	NewWorker(s, &fakeProvider{inspoResult: bad}, photosDir, 0).ProcessOne(ctx)

	job, _ := s.JobByID(ctx, jobID)
	if job.Status != "pending" || job.Attempts != 1 {
		t.Errorf("job = %s, attempt %d; want rescheduled after an invalid answer", job.Status, job.Attempts)
	}
	if fit, _ := s.InspoFitByID(ctx, userID, fitID); len(fit.Styles) != 0 {
		t.Errorf("invalid styles were saved: %v", fit.Styles)
	}
}

func TestSummarizeStyleOnlyWhenProfileChanged(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, fitID := newInspoFixture(t)
	if err := s.UpdateInspoTags(ctx, userID, fitID, store.InspoTags{Styles: []string{"minimal"}}); err != nil {
		t.Fatalf("UpdateInspoTags: %v", err)
	}
	provider := &fakeProvider{summary: "Clean and quiet."}
	w := NewWorker(s, provider, photosDir, 0)

	run := func() {
		t.Helper()
		id, _ := s.EnqueueJob(ctx, SummarizeStyleType, SummarizeStylePayload{UserID: userID})
		w.ProcessOne(ctx)
		if job, _ := s.JobByID(ctx, id); job.Status != "done" {
			t.Fatalf("summary job = %s (%s), want done", job.Status, job.LastError)
		}
	}

	run()
	if summary, _, _ := s.StyleSummary(ctx, userID); summary != "Clean and quiet." || provider.summaries != 1 {
		t.Fatalf("summary = %q after %d calls, want written once", summary, provider.summaries)
	}

	run() // same profile: no new call
	if provider.summaries != 1 {
		t.Errorf("SummarizeStyle called %d times for an unchanged profile, want 1", provider.summaries)
	}

	s.UpdateInspoTags(ctx, userID, fitID, store.InspoTags{Styles: []string{"edgy"}})
	run()
	if provider.summaries != 2 {
		t.Errorf("SummarizeStyle called %d times after the profile changed, want 2", provider.summaries)
	}
}
