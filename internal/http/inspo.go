package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gianlucanotaro/wardy/internal/inspo"
	"github.com/gianlucanotaro/wardy/internal/jobs"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// The inspiration board (#49): photos of other people's outfits, tagged,
// adding up to a style profile that outfit suggestions use as a
// preference. Same shape as items — photos stored and served per owner,
// AI tagging as a background job — but its own table and tags.

type inspoFitResponse struct {
	store.InspoFit
	TaggingPending bool `json:"tagging_pending"`
}

type inspoBoardResponse struct {
	Fits    []inspoFitResponse `json:"fits"`
	Profile inspo.Profile      `json:"profile"`
	// Summary is the AI's one-line description of the profile; empty
	// until one has been written for the current profile, or with AI off.
	Summary string `json:"summary"`
	// SummaryPending: a rewrite of Summary is queued, so the board is
	// worth polling until it lands.
	SummaryPending   bool `json:"summary_pending"`
	UseInSuggestions bool `json:"use_in_suggestions"`
	MinFits          int  `json:"min_fits"`
}

// inspoBoardHandler returns the whole board: every fit, the profile they
// add up to, and the cached summary — one request for the INSPO screen.
func inspoBoardHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		fits, err := s.InspoFitsByUser(r.Context(), user.ID)
		if err != nil {
			slog.Error("inspo board", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		pending, err := s.PendingTagInspoIDs(r.Context())
		if err != nil {
			slog.Error("inspo board: check pending tag jobs", "error", err)
			pending = map[int64]bool{}
		}
		profile := inspo.Build(fits)

		resp := inspoBoardResponse{
			Fits:             make([]inspoFitResponse, len(fits)),
			Profile:          profile,
			UseInSuggestions: user.UseInspo,
			MinFits:          inspo.MinFitsForSuggestions,
		}
		for i, f := range fits {
			resp.Fits[i] = inspoFitResponse{InspoFit: f, TaggingPending: pending[f.ID]}
		}
		// A summary written for an older profile would contradict the
		// counts shown next to it, so it's left out until the rewrite
		// (SummaryPending) lands.
		if summary, key, err := s.StyleSummary(r.Context(), user.ID); err != nil {
			slog.Error("inspo board: style summary", "error", err)
		} else if profile.Tagged > 0 && key == profile.Key() {
			resp.Summary = summary
		}
		resp.SummaryPending, err = s.HasPendingJob(r.Context(), jobs.SummarizeStyleType, jobs.SummarizeStylePayload{UserID: user.ID})
		if err != nil {
			slog.Error("inspo board: check pending summary", "error", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// uploadInspoHandler stores a new fit's photo and queues its AI tagging,
// under the same kill switch and daily cap as item uploads.
func uploadInspoHandler(s *store.Store, photosDir string, aiEnabled bool, jobLimiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		photoRel, thumbRel, ok := savePhoto(w, r, photosDir, user.ID)
		if !ok {
			return
		}

		fitID, err := s.CreateInspoFit(r.Context(), user.ID, photoRel, thumbRel)
		if errors.Is(err, store.ErrLimitReached) {
			removePhotos(photosDir, photoRel, thumbRel)
			http.Error(w, "your inspo board is full — delete a few fits first", http.StatusConflict)
			return
		}
		if err != nil {
			slog.Error("inspo upload: create fit", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if aiEnabled && jobLimiter.Allow(strconv.FormatInt(user.ID, 10)) {
			payload := jobs.TagInspoPayload{FitID: fitID, UserID: user.ID}
			if _, err := s.EnqueueJob(r.Context(), jobs.TagInspoType, payload); err != nil {
				slog.Error("inspo upload: enqueue tag_inspo job", "error", err)
			}
		}

		writeInspoFit(r.Context(), w, s, user.ID, fitID, http.StatusCreated)
	}
}

// updateInspoTagsHandler applies a full manual tag edit, validated against
// the closed vocabularies before anything is written.
func updateInspoTagsHandler(s *store.Store, aiEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid fit id", http.StatusBadRequest)
			return
		}

		var req struct {
			Styles       []string `json:"styles"`
			Colors       []string `json:"colors"`
			Pieces       []string `json:"pieces"`
			Fit          string   `json:"fit"`
			FormalityMin int      `json:"formality_min"`
			FormalityMax int      `json:"formality_max"`
			Seasons      []string `json:"seasons"`
			Occasions    []string `json:"occasions"`
			// Free-form: only store's count/length caps apply.
			CustomTags []string `json:"custom_tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		tags := taxonomy.InspoTags{
			Styles: req.Styles, Colors: req.Colors, Pieces: req.Pieces, Fit: req.Fit,
			FormalityMin: req.FormalityMin, FormalityMax: req.FormalityMax,
			Seasons: req.Seasons, Occasions: req.Occasions,
		}
		if err := tags.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateInspoTags(r.Context(), user.ID, id, store.InspoTags{
			Styles: tags.Styles, Colors: tags.Colors, Pieces: tags.Pieces, Fit: tags.Fit,
			FormalityMin: tags.FormalityMin, FormalityMax: tags.FormalityMax,
			Seasons: tags.Seasons, Occasions: tags.Occasions, CustomTags: req.CustomTags,
		})
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("update inspo tags", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		queueStyleSummary(r.Context(), s, aiEnabled, user.ID)
		writeInspoFit(r.Context(), w, s, user.ID, id, http.StatusOK)
	}
}

func deleteInspoHandler(s *store.Store, photosDir string, aiEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid fit id", http.StatusBadRequest)
			return
		}

		photoRel, thumbRel, err := s.DeleteInspoFit(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("delete inspo fit", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		removePhotos(photosDir, photoRel, thumbRel)

		queueStyleSummary(r.Context(), s, aiEnabled, user.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// inspoSettingsHandler turns the style profile on or off for suggestions.
func inspoSettingsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			UseInSuggestions *bool `json:"use_in_suggestions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UseInSuggestions == nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := s.SetUseInspo(r.Context(), user.ID, *req.UseInSuggestions); err != nil {
			slog.Error("inspo settings", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"use_in_suggestions": *req.UseInSuggestions})
	}
}

// serveInspoPhotoHandler serves a fit's photo or thumbnail to its owner
// only — like servePhotoHandler, this check is all that keeps one user's
// board from another.
func serveInspoPhotoHandler(s *store.Store, photosDir string, thumb bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid fit id", http.StatusBadRequest)
			return
		}

		fit, err := s.InspoFitByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("serve inspo photo", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		rel := fit.PhotoPath
		if thumb {
			rel = fit.ThumbPath
		}
		http.ServeFile(w, r, filepath.Join(photosDir, rel))
	}
}

func writeInspoFit(ctx context.Context, w http.ResponseWriter, s *store.Store, userID, id int64, statusCode int) {
	fit, err := s.InspoFitByID(ctx, userID, id)
	if err != nil {
		slog.Error("inspo: fetch fit", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pending, err := s.PendingTagInspoIDs(ctx)
	if err != nil {
		slog.Error("inspo: check pending tag jobs", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(inspoFitResponse{InspoFit: *fit, TaggingPending: pending[id]})
}

// queueStyleSummary asks the worker to rewrite the style summary after a
// change to the board. With AI off there's nothing to write it.
func queueStyleSummary(ctx context.Context, s *store.Store, aiEnabled bool, userID int64) {
	if !aiEnabled {
		return
	}
	if err := jobs.EnqueueStyleSummary(ctx, s, userID); err != nil {
		slog.Error("inspo: enqueue style summary", "error", err)
	}
}
