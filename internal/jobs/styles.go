package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// TagItemStylesType tags only the styles of an item uploaded before items
// had styles (migration 0008). New items get their styles from tag_item.
const TagItemStylesType = "tag_item_styles"

// EnqueueStyleBackfill queues a tag_item_styles job for every item the AI
// has never looked at for styles and that has none, skipping ones already
// queued. Called once at startup: after the first full run every item is
// marked looked at, so later starts queue nothing. The worker takes one
// job per poll, so a whole wardrobe trickles through in a few minutes, and
// the monthly AI budget still applies to each job.
func EnqueueStyleBackfill(ctx context.Context, s *store.Store) (int, error) {
	refs, err := s.ItemsNeedingStyles(ctx)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, r := range refs {
		payload := TagItemPayload{ItemID: r.ID, UserID: r.UserID}
		pending, err := s.HasPendingJob(ctx, TagItemStylesType, payload)
		if err != nil {
			return queued, err
		}
		if pending {
			continue
		}
		if _, err := s.EnqueueJob(ctx, TagItemStylesType, payload); err != nil {
			return queued, err
		}
		queued++
	}
	return queued, nil
}

// runTagItemStyles tags one item's styles and saves them, unless the user
// set styles by hand meanwhile (UpdateItemAIStyles' guard).
func (w *Worker) runTagItemStyles(ctx context.Context, job *store.Job) error {
	var payload TagItemPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse job payload: %w", err)
	}

	item, err := w.store.ItemByID(ctx, payload.UserID, payload.ItemID)
	if errors.Is(err, store.ErrNotFound) {
		return nil // deleted since it was queued
	}
	if err != nil {
		return fmt.Errorf("look up item: %w", err)
	}

	thumb, err := os.ReadFile(filepath.Join(w.photosDir, item.ThumbPath))
	if err != nil {
		return fmt.Errorf("read thumbnail: %w", err)
	}

	styles, usage, err := w.provider.TagItemStyles(ctx, thumb)
	w.logUsage(ctx, payload.UserID, usage)
	if errors.Is(err, ai.ErrDisabled) {
		return nil // AI turned off; the item stays queued-able for later
	}
	if err != nil {
		return fmt.Errorf("tag item styles: %w", err)
	}
	if err := taxonomy.ValidItemStyles(styles); err != nil {
		return fmt.Errorf("model returned invalid styles: %w", err)
	}

	if _, err := w.store.UpdateItemAIStyles(ctx, payload.ItemID, styles); err != nil {
		return fmt.Errorf("save ai styles: %w", err)
	}
	return nil
}
