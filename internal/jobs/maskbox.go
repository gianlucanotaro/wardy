package jobs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gianlucanotaro/wardy/internal/images"
	"github.com/gianlucanotaro/wardy/internal/store"
)

// BackfillMaskBoxes measures where the garment sits in every cut-out that has
// no box yet (a cut-out made before migration 0010), so the app can give it
// the same size as the others. Called at startup: a mask file is a few tens of
// KB, so a whole wardrobe takes a moment, and once every one has a box later
// starts find nothing to do. A mask that cannot be read or measured is logged
// and left, never a reason not to start. Returns how many it measured.
func BackfillMaskBoxes(ctx context.Context, s *store.Store, photosDir string) (int, error) {
	refs, err := s.ItemsNeedingMaskBox(ctx)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range refs {
		data, err := os.ReadFile(filepath.Join(photosDir, r.MaskPath))
		if err != nil {
			slog.Warn("mask box backfill: read mask", "item", r.ID, "error", err)
			continue
		}
		box, err := images.MeasureMask(data)
		if err != nil {
			slog.Warn("mask box backfill: measure mask", "item", r.ID, "error", err)
			continue
		}
		if err := s.SetItemMaskBox(ctx, r.ID, r.MaskPath, store.MaybeBox{Box: box, Valid: true}); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}
