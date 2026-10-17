package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/inspo"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// runTagInspo tags one inspiration fit, then queues a style summary
// rewrite, since the profile just changed.
func (w *Worker) runTagInspo(ctx context.Context, job *store.Job) error {
	var payload TagInspoPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse job payload: %w", err)
	}

	fit, err := w.store.InspoFitByID(ctx, payload.UserID, payload.FitID)
	if errors.Is(err, store.ErrNotFound) {
		return nil // deleted since it was queued: nothing left to tag
	}
	if err != nil {
		return fmt.Errorf("look up inspo fit: %w", err)
	}

	thumb, err := os.ReadFile(filepath.Join(w.photosDir, fit.ThumbPath))
	if err != nil {
		return fmt.Errorf("read thumbnail: %w", err)
	}

	result, usage, err := w.provider.TagInspo(ctx, thumb)
	w.logUsage(ctx, payload.UserID, usage)
	if errors.Is(err, ai.ErrDisabled) {
		return nil // AI turned off; left for manual tagging
	}
	if err != nil {
		return fmt.Errorf("tag inspo: %w", err)
	}

	tags := taxonomy.InspoTags{
		Styles: result.Styles, Colors: result.Colors, Pieces: result.Pieces, Fit: result.Fit,
		FormalityMin: result.FormalityMin, FormalityMax: result.FormalityMax,
		Seasons: result.Seasons, Occasions: result.Occasions,
	}
	if err := tags.Validate(); err != nil {
		return fmt.Errorf("model returned invalid inspo tags: %w", err)
	}

	applied, err := w.store.UpdateInspoAITags(ctx, payload.FitID, store.InspoTags{
		Styles: tags.Styles, Colors: tags.Colors, Pieces: tags.Pieces, Fit: tags.Fit,
		FormalityMin: tags.FormalityMin, FormalityMax: tags.FormalityMax,
		Seasons: tags.Seasons, Occasions: tags.Occasions,
	})
	if err != nil {
		return fmt.Errorf("save inspo ai tags: %w", err)
	}
	if applied {
		if err := EnqueueStyleSummary(ctx, w.store, payload.UserID); err != nil {
			slog.Error("jobs: enqueue style summary", "error", err)
		}
	}
	return nil
}

// runSummarizeStyle rewrites a user's style summary — but only when their
// profile changed since the last one was written (Profile.Key), so edits
// that don't move the profile cost nothing.
func (w *Worker) runSummarizeStyle(ctx context.Context, job *store.Job) error {
	var payload SummarizeStylePayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse job payload: %w", err)
	}

	fits, err := w.store.InspoFitsByUser(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("list inspo fits: %w", err)
	}
	profile := inspo.Build(fits)
	key := profile.Key()

	_, cachedKey, err := w.store.StyleSummary(ctx, payload.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil // user deleted
	}
	if err != nil {
		return fmt.Errorf("look up style summary: %w", err)
	}
	if key == cachedKey {
		return nil
	}
	if profile.Tagged == 0 {
		return w.store.SetStyleSummary(ctx, payload.UserID, "", key)
	}

	summary, usage, err := w.provider.SummarizeStyle(ctx, profile.Describe())
	w.logUsage(ctx, payload.UserID, usage)
	if errors.Is(err, ai.ErrDisabled) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("summarize style: %w", err)
	}
	return w.store.SetStyleSummary(ctx, payload.UserID, summary, key)
}

func (w *Worker) logUsage(ctx context.Context, userID int64, usage ai.Usage) {
	if usage.Model == "" {
		return
	}
	if err := w.store.LogUsage(ctx, userID, usage.Model, usage.InputTokens, usage.OutputTokens, usage.EstCostUSD); err != nil {
		slog.Error("jobs: log usage", "error", err)
	}
}
