package jobs

import (
	"context"
	"reflect"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/ai"
)

func TestStyleBackfillTagsOnceAndOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)

	n, err := EnqueueStyleBackfill(ctx, s)
	if err != nil || n != 1 {
		t.Fatalf("EnqueueStyleBackfill = %d, %v; want 1 queued", n, err)
	}
	// A second start before the job ran doesn't queue a duplicate.
	if n, err := EnqueueStyleBackfill(ctx, s); err != nil || n != 0 {
		t.Fatalf("second EnqueueStyleBackfill = %d, %v; want 0", n, err)
	}

	provider := &fakeProvider{styles: []string{"workwear", "minimal"}, usage: ai.Usage{Model: "fake"}}
	NewWorker(s, provider, photosDir, 0).ProcessOne(ctx)

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if !reflect.DeepEqual([]string(item.Styles), []string{"workwear", "minimal"}) {
		t.Errorf("Styles = %v, want [workwear minimal]", item.Styles)
	}
	// Every item is marked looked at, so a later start queues nothing.
	if n, err := EnqueueStyleBackfill(ctx, s); err != nil || n != 0 {
		t.Errorf("EnqueueStyleBackfill after the run = %d, %v; want 0", n, err)
	}
}

func TestStyleBackfillRetriesOutOfVocabStyles(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, itemID := newTestFixture(t)
	if _, err := EnqueueStyleBackfill(ctx, s); err != nil {
		t.Fatal(err)
	}

	provider := &fakeProvider{styles: []string{"cottagecore"}}
	NewWorker(s, provider, photosDir, 0).ProcessOne(ctx)

	item, _ := s.ItemByID(ctx, userID, itemID)
	if len(item.Styles) != 0 {
		t.Errorf("Styles = %v, want none saved from an invalid answer", item.Styles)
	}
}
