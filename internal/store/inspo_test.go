package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestInspoFitOwnership: user B can't read, edit or delete user A's fit.
func TestInspoFitOwnership(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")

	fitID, err := s.CreateInspoFit(ctx, userA, "1/p.jpg", "1/t.jpg")
	if err != nil {
		t.Fatalf("CreateInspoFit: %v", err)
	}

	if _, err := s.InspoFitByID(ctx, userB, fitID); !errors.Is(err, ErrNotFound) {
		t.Errorf("InspoFitByID as B = %v, want ErrNotFound", err)
	}
	if fits, _ := s.InspoFitsByUser(ctx, userB); len(fits) != 0 {
		t.Errorf("B's board has %d fits, want 0", len(fits))
	}
	if err := s.UpdateInspoTags(ctx, userB, fitID, InspoTags{Styles: []string{"edgy"}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateInspoTags as B = %v, want ErrNotFound", err)
	}
	if _, _, err := s.DeleteInspoFit(ctx, userB, fitID); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteInspoFit as B = %v, want ErrNotFound", err)
	}

	fit, err := s.InspoFitByID(ctx, userA, fitID)
	if err != nil || len(fit.Styles) != 0 {
		t.Fatalf("A's fit after B's attempts = %+v, %v; want untouched", fit, err)
	}
	photo, thumb, err := s.DeleteInspoFit(ctx, userA, fitID)
	if err != nil || photo != "1/p.jpg" || thumb != "1/t.jpg" {
		t.Errorf("DeleteInspoFit as A = %q, %q, %v", photo, thumb, err)
	}
}

// TestInspoAITagsNeverOverwriteUserEdits, as for items (docs/ai.md).
func TestInspoAITagsNeverOverwriteUserEdits(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	user, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")

	fitID, _ := s.CreateInspoFit(ctx, user, "p.jpg", "t.jpg")
	applied, err := s.UpdateInspoAITags(ctx, fitID, InspoTags{Styles: []string{"minimal"}})
	if err != nil || !applied {
		t.Fatalf("UpdateInspoAITags on an untouched fit = %v, %v; want applied", applied, err)
	}

	if err := s.UpdateInspoTags(ctx, user, fitID, InspoTags{Styles: []string{"edgy"}, CustomTags: []string{"  mine  "}}); err != nil {
		t.Fatalf("UpdateInspoTags: %v", err)
	}
	applied, err = s.UpdateInspoAITags(ctx, fitID, InspoTags{Styles: []string{"preppy"}})
	if err != nil || applied {
		t.Errorf("UpdateInspoAITags after a user edit = %v, %v; want not applied", applied, err)
	}

	fit, _ := s.InspoFitByID(ctx, user, fitID)
	if len(fit.Styles) != 1 || fit.Styles[0] != "edgy" {
		t.Errorf("styles = %v, want [edgy]", fit.Styles)
	}
	if len(fit.CustomTags) != 1 || fit.CustomTags[0] != "mine" {
		t.Errorf("custom tags = %v, want [mine] (trimmed)", fit.CustomTags)
	}
	if !fit.AITaggedAt.Valid || !fit.UserEditedAt.Valid {
		t.Errorf("ai_tagged_at, user_edited_at = %v, %v; want both set", fit.AITaggedAt, fit.UserEditedAt)
	}
}

func TestCreateInspoFitLimit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	user, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	for range MaxInspoFits {
		if _, err := s.CreateInspoFit(ctx, user, "p.jpg", "t.jpg"); err != nil {
			t.Fatalf("CreateInspoFit: %v", err)
		}
	}
	if _, err := s.CreateInspoFit(ctx, user, "p.jpg", "t.jpg"); !errors.Is(err, ErrLimitReached) {
		t.Errorf("CreateInspoFit past the cap = %v, want ErrLimitReached", err)
	}
}

func TestUseInspoAndStyleSummary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")

	u, _ := s.UserByID(ctx, id)
	if !u.UseInspo {
		t.Error("UseInspo on a new user = false, want true")
	}
	if err := s.SetUseInspo(ctx, id, false); err != nil {
		t.Fatalf("SetUseInspo: %v", err)
	}
	if u, _ := s.UserByID(ctx, id); u.UseInspo {
		t.Error("UseInspo after turning it off = true")
	}

	if err := s.SetStyleSummary(ctx, id, "Clean and muted.", "k1"); err != nil {
		t.Fatalf("SetStyleSummary: %v", err)
	}
	summary, key, err := s.StyleSummary(ctx, id)
	if err != nil || summary != "Clean and muted." || key != "k1" {
		t.Errorf("StyleSummary = %q, %q, %v", summary, key, err)
	}
}

// TestEnqueueJobOnce: a second identical job pushes the pending one back
// instead of queueing another; a different payload is its own job.
func TestEnqueueJobOnce(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	type payload struct {
		UserID int64 `json:"user_id"`
	}
	if err := s.EnqueueJobOnce(ctx, "summarize_style", payload{1}, 0); err != nil {
		t.Fatalf("EnqueueJobOnce: %v", err)
	}
	if err := s.EnqueueJobOnce(ctx, "summarize_style", payload{1}, time.Hour); err != nil {
		t.Fatalf("EnqueueJobOnce again: %v", err)
	}
	if err := s.EnqueueJobOnce(ctx, "summarize_style", payload{2}, 0); err != nil {
		t.Fatalf("EnqueueJobOnce other user: %v", err)
	}

	job, err := s.ClaimNextJob(ctx, "summarize_style")
	if err != nil {
		t.Fatalf("ClaimNextJob: %v", err)
	}
	if job.Payload != `{"user_id":2}` {
		t.Errorf("first due job = %s, want user 2's (user 1's was pushed back an hour)", job.Payload)
	}
	s.CompleteJob(ctx, job.ID)
	if _, err := s.ClaimNextJob(ctx, "summarize_style"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClaimNextJob = %v, want ErrNotFound: user 1's single job isn't due", err)
	}
}

func TestClaimNextJobOfSeveralTypes(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.EnqueueJob(ctx, "other", struct{}{})
	id, _ := s.EnqueueJob(ctx, "tag_inspo", struct{}{})

	job, err := s.ClaimNextJob(ctx, "tag_item", "tag_inspo")
	if err != nil || job.ID != id {
		t.Errorf("ClaimNextJob(tag_item, tag_inspo) = %+v, %v; want the tag_inspo job", job, err)
	}
}
