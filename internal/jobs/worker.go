// Package jobs runs the background worker that processes the jobs table:
// tag_item (AI vision tagging of a garment), and for the inspiration board
// (#49) tag_inspo and summarize_style. No Redis, no external queue — one
// goroutine polling every few seconds is plenty.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

const (
	pollInterval = 3 * time.Second

	// TagItemType is the job type created on every photo upload.
	TagItemType = "tag_item"

	// TagInspoType is created on every inspiration photo upload.
	TagInspoType = "tag_inspo"

	// SummarizeStyleType rewrites a user's style summary. Queued with
	// EnqueueStyleSummary after anything that can change their profile.
	SummarizeStyleType = "summarize_style"

	// styleSummaryDelay debounces SummarizeStyleType: tagging ten fits
	// in a row rewrites the summary once, after the last.
	styleSummaryDelay = 20 * time.Second
)

// TagInspoPayload is the job payload the inspo upload handler enqueues.
type TagInspoPayload struct {
	FitID  int64 `json:"fit_id"`
	UserID int64 `json:"user_id"`
}

// SummarizeStylePayload identifies whose style summary to rewrite.
type SummarizeStylePayload struct {
	UserID int64 `json:"user_id"`
}

// EnqueueStyleSummary queues (or pushes back) a rewrite of userID's style
// summary. The job itself checks whether the profile actually changed, so
// calling this too often only costs a cheap no-op run.
func EnqueueStyleSummary(ctx context.Context, s *store.Store, userID int64) error {
	return s.EnqueueJobOnce(ctx, SummarizeStyleType, SummarizeStylePayload{UserID: userID}, styleSummaryDelay)
}

// TagItemPayload is the job payload the upload handler enqueues.
type TagItemPayload struct {
	ItemID int64 `json:"item_id"`
	UserID int64 `json:"user_id"`
}

// Worker polls the jobs table and processes tag_item jobs.
type Worker struct {
	store            *store.Store
	provider         ai.Provider
	photosDir        string
	monthlyBudgetUSD float64
}

func NewWorker(s *store.Store, provider ai.Provider, photosDir string, monthlyBudgetUSD float64) *Worker {
	return &Worker{
		store:            s,
		provider:         provider,
		photosDir:        photosDir,
		monthlyBudgetUSD: monthlyBudgetUSD,
	}
}

// Run polls until ctx is done. Meant to be called in its own goroutine.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ProcessOne(ctx)
		}
	}
}

// ProcessOne claims and processes a single job, if one is ready.
// Exported so tests can drive it directly rather than through Run's ticker.
func (w *Worker) ProcessOne(ctx context.Context) {
	job, err := w.store.ClaimNextJob(ctx, TagItemType, TagInspoType, SummarizeStyleType)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		slog.Error("jobs: claim", "error", err)
		return
	}

	if w.monthlyBudgetUSD > 0 {
		spend, err := w.store.MonthlySpend(ctx)
		if err != nil {
			slog.Error("jobs: check budget", "error", err)
			w.retry(ctx, job, err)
			return
		}
		if spend >= w.monthlyBudgetUSD {
			msg := fmt.Sprintf("monthly AI budget exceeded ($%.2f >= $%.2f)", spend, w.monthlyBudgetUSD)
			slog.Warn("jobs: skipping job, over budget", "type", job.Type, "job_id", job.ID)
			if err := w.store.FailJobPermanently(ctx, job.ID, msg); err != nil {
				slog.Error("jobs: record permanent failure", "error", err)
			}
			return
		}
	}

	if err := w.run(ctx, job); err != nil {
		if ai.IsRetryable(err) {
			w.retry(ctx, job, err)
		} else {
			slog.Warn("jobs: permanently failed (non-retryable)", "type", job.Type, "job_id", job.ID, "error", err)
			if failErr := w.store.FailJobPermanently(ctx, job.ID, err.Error()); failErr != nil {
				slog.Error("jobs: record permanent failure", "error", failErr)
			}
		}
		return
	}

	if err := w.store.CompleteJob(ctx, job.ID); err != nil {
		slog.Error("jobs: complete", "error", err)
	}
}

func (w *Worker) run(ctx context.Context, job *store.Job) error {
	switch job.Type {
	case TagInspoType:
		return w.runTagInspo(ctx, job)
	case SummarizeStyleType:
		return w.runSummarizeStyle(ctx, job)
	default:
		return w.runTagItem(ctx, job)
	}
}

func (w *Worker) retry(ctx context.Context, job *store.Job, cause error) {
	slog.Warn("jobs: failed, will retry", "type", job.Type, "job_id", job.ID, "attempts", job.Attempts, "error", cause)
	if err := w.store.FailJob(ctx, job.ID, job.Attempts, cause.Error()); err != nil {
		slog.Error("jobs: record failure", "error", err)
	}
}

func (w *Worker) runTagItem(ctx context.Context, job *store.Job) error {
	var payload TagItemPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse job payload: %w", err)
	}

	item, err := w.store.ItemByID(ctx, payload.UserID, payload.ItemID)
	if errors.Is(err, store.ErrNotFound) {
		// Item deleted (or reassigned) since the job was queued — nothing
		// left to tag. Treat as done rather than retrying forever.
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up item: %w", err)
	}

	thumbBytes, err := os.ReadFile(filepath.Join(w.photosDir, item.ThumbPath))
	if err != nil {
		return fmt.Errorf("read thumbnail: %w", err)
	}

	result, usage, err := w.provider.TagItem(ctx, thumbBytes)
	if usage.Model != "" {
		if logErr := w.store.LogUsage(ctx, payload.UserID, usage.Model, usage.InputTokens, usage.OutputTokens, usage.EstCostUSD); logErr != nil {
			slog.Error("jobs: log usage", "error", logErr)
		}
	}
	if errors.Is(err, ai.ErrDisabled) {
		return nil // AI turned off; leave the item for manual tagging.
	}
	if err != nil {
		return fmt.Errorf("tag item: %w", err)
	}

	tags := taxonomy.Tags{
		Category:       taxonomy.Category(result.Category),
		Subcategory:    result.Subcategory,
		ColorPrimary:   result.ColorPrimary,
		ColorSecondary: result.ColorSecondary,
		Pattern:        taxonomy.Pattern(result.Pattern),
		Material:       taxonomy.Material(result.Material),
		Fit:            taxonomy.Fit(result.Fit),
		FormalityMin:   result.FormalityMin,
		FormalityMax:   result.FormalityMax,
		WarmthMin:      result.WarmthMin,
		WarmthMax:      result.WarmthMax,
		Seasons:        result.Seasons,
		Occasions:      result.Occasions,
	}
	if err := tags.Validate(); err != nil {
		return fmt.Errorf("model returned invalid tags: %w", err)
	}

	if _, err := w.store.UpdateItemAITags(ctx, payload.ItemID, store.ItemTags{
		Category:       string(tags.Category),
		Subcategory:    tags.Subcategory,
		ColorPrimary:   tags.ColorPrimary,
		ColorSecondary: tags.ColorSecondary,
		Pattern:        string(tags.Pattern),
		Material:       string(tags.Material),
		Fit:            string(tags.Fit),
		FormalityMin:   tags.FormalityMin,
		FormalityMax:   tags.FormalityMax,
		WarmthMin:      tags.WarmthMin,
		WarmthMax:      tags.WarmthMax,
		Seasons:        tags.Seasons,
		Occasions:      tags.Occasions,
		CustomTags:     result.CustomTags,
	}); err != nil {
		return fmt.Errorf("save ai tags: %w", err)
	}

	return nil
}
