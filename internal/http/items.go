package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	"github.com/gianlucanotaro/wardy/internal/images"
	"github.com/gianlucanotaro/wardy/internal/jobs"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// maxUploadBytes caps the raw request body, not just the decoded image —
// oversized uploads get rejected before any image processing happens.
const maxUploadBytes = 15 << 20 // 15 MB

// itemResponse adds a derived tagging_pending flag to store.Item's own
// columns, so the frontend knows whether to keep polling (a tag_item job
// is still pending) or stop (tagged, permanently failed, or never
// queued all mean "stop" — see docs/frontend.md's upload/tagging flow).
type itemResponse struct {
	store.Item
	TaggingPending bool `json:"tagging_pending"`
}

func writeItem(ctx context.Context, w http.ResponseWriter, s *store.Store, item *store.Item, statusCode int) {
	pending, err := s.ItemHasPendingTagJob(ctx, item.ID)
	if err != nil {
		slog.Error("check pending tag job", "error", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(itemResponse{Item: *item, TaggingPending: pending})
}

// uploadItemHandler accepts a multipart "photo" field, runs it through the
// image pipeline (orientation fix, resize, EXIF strip), stores both
// derivatives on disk, and creates the item row.
func uploadItemHandler(s *store.Store, photosDir string, aiEnabled bool, jobLimiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			http.Error(w, "photo too large or malformed form", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("photo")
		if err != nil {
			http.Error(w, "missing \"photo\" field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		data := make([]byte, 0, 1<<20)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := file.Read(buf)
			data = append(data, buf[:n]...)
			if readErr != nil {
				break
			}
		}

		display, thumb, err := images.Process(data)
		if errors.Is(err, images.ErrHEIC) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			slog.Error("upload: process image", "error", err)
			http.Error(w, "unsupported or corrupt image", http.StatusBadRequest)
			return
		}

		userDir := filepath.Join(photosDir, strconv.FormatInt(user.ID, 10))
		if err := os.MkdirAll(userDir, 0o755); err != nil {
			slog.Error("upload: create photo dir", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		id := uuid.NewString()
		photoRel := filepath.Join(strconv.FormatInt(user.ID, 10), id+".jpg")
		thumbRel := filepath.Join(strconv.FormatInt(user.ID, 10), id+"_thumb.jpg")

		if err := os.WriteFile(filepath.Join(photosDir, photoRel), display, 0o644); err != nil {
			slog.Error("upload: write photo", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(filepath.Join(photosDir, thumbRel), thumb, 0o644); err != nil {
			slog.Error("upload: write thumb", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		itemID, err := s.CreateItem(r.Context(), user.ID, photoRel, thumbRel)
		if err != nil {
			slog.Error("upload: create item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Enqueue AI tagging unless the kill switch is off or this user has
		// hit today's cap — either way the item is still fully usable, just
		// left for manual tagging (see docs/ai.md's budget guards).
		if aiEnabled && jobLimiter.Allow(strconv.FormatInt(user.ID, 10)) {
			payload := jobs.TagItemPayload{ItemID: itemID, UserID: user.ID}
			if _, err := s.EnqueueJob(r.Context(), jobs.TagItemType, payload); err != nil {
				slog.Error("upload: enqueue tag_item job", "error", err)
			}
		}

		item, err := s.ItemByID(r.Context(), user.ID, itemID)
		if err != nil {
			slog.Error("upload: fetch created item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeItem(r.Context(), w, s, item, http.StatusCreated)
	}
}

// itemWithWearStatsResponse adds the same derived tagging_pending flag
// itemResponse does, but on top of ItemsByUser's wear-count/last-worn
// columns rather than plain store.Item — see itemResponse for why.
type itemWithWearStatsResponse struct {
	store.ItemWithWearStats
	TaggingPending bool `json:"tagging_pending"`
}

func listItemsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		archived := r.URL.Query().Get("archived") == "true"
		items, err := s.ItemsByUser(r.Context(), user.ID, archived)
		if err != nil {
			slog.Error("list items", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		pending, err := s.PendingTagItemIDs(r.Context())
		if err != nil {
			slog.Error("list items: check pending tag jobs", "error", err)
			pending = map[int64]bool{}
		}

		responses := make([]itemWithWearStatsResponse, len(items))
		for i, item := range items {
			responses[i] = itemWithWearStatsResponse{ItemWithWearStats: item, TaggingPending: pending[item.ID]}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

func getItemHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		item, err := s.ItemByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("get item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeItem(r.Context(), w, s, item, http.StatusOK)
	}
}

func updateItemHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if err := s.UpdateItemName(r.Context(), user.ID, id, req.Name); errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			slog.Error("update item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		item, err := s.ItemByID(r.Context(), user.ID, id)
		if err != nil {
			slog.Error("update item: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeItem(r.Context(), w, s, item, http.StatusOK)
	}
}

// archiveItemHandler sets or clears an item's archived flag — the
// reversible alternative to deleteItemHandler's permanent removal.
func archiveItemHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		var req struct {
			Archived bool `json:"archived"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if err := s.ArchiveItem(r.Context(), user.ID, id, req.Archived); errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			slog.Error("archive item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		item, err := s.ItemByID(r.Context(), user.ID, id)
		if err != nil {
			slog.Error("archive item: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeItem(r.Context(), w, s, item, http.StatusOK)
	}
}

// updateItemTagsHandler applies a full manual tag edit. Every field is
// validated against the closed vocabularies in internal/taxonomy before
// anything is written — a partially-invalid payload is rejected outright
// rather than partially applied.
func updateItemTagsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		var req struct {
			Category       string   `json:"category"`
			Subcategory    string   `json:"subcategory"`
			ColorPrimary   string   `json:"color_primary"`
			ColorSecondary string   `json:"color_secondary"`
			Pattern        string   `json:"pattern"`
			Material       string   `json:"material"`
			Fit            string   `json:"fit"`
			FormalityMin   int      `json:"formality_min"`
			FormalityMax   int      `json:"formality_max"`
			WarmthMin      int      `json:"warmth_min"`
			WarmthMax      int      `json:"warmth_max"`
			Seasons        []string `json:"seasons"`
			Occasions      []string `json:"occasions"`
			// Not part of the closed taxonomy — no Validate() below,
			// only the store layer's count/length caps (store.ItemTags).
			CustomTags []string `json:"custom_tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		tags := taxonomy.Tags{
			Category:       taxonomy.Category(req.Category),
			Subcategory:    req.Subcategory,
			ColorPrimary:   req.ColorPrimary,
			ColorSecondary: req.ColorSecondary,
			Pattern:        taxonomy.Pattern(req.Pattern),
			Material:       taxonomy.Material(req.Material),
			Fit:            taxonomy.Fit(req.Fit),
			FormalityMin:   req.FormalityMin,
			FormalityMax:   req.FormalityMax,
			WarmthMin:      req.WarmthMin,
			WarmthMax:      req.WarmthMax,
			Seasons:        req.Seasons,
			Occasions:      req.Occasions,
		}
		if err := tags.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateItemTags(r.Context(), user.ID, id, store.ItemTags{
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
			CustomTags:     req.CustomTags,
		})
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("update item tags", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		item, err := s.ItemByID(r.Context(), user.ID, id)
		if err != nil {
			slog.Error("update item tags: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeItem(r.Context(), w, s, item, http.StatusOK)
	}
}

func deleteItemHandler(s *store.Store, photosDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		photoRel, thumbRel, err := s.DeleteItem(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("delete item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		for _, rel := range []string{photoRel, thumbRel} {
			if err := os.Remove(filepath.Join(photosDir, rel)); err != nil && !os.IsNotExist(err) {
				slog.Warn("delete item: remove file", "path", rel, "error", err)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// servePhotoHandler serves an item's photo or thumbnail, scoped to the
// authenticated owner — there is no static file mount for the photos
// directory, so this ownership check is the only thing standing between a
// request and another user's photo.
func servePhotoHandler(s *store.Store, photosDir string, thumb bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid item id", http.StatusBadRequest)
			return
		}

		item, err := s.ItemByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("serve photo", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		rel := item.PhotoPath
		if thumb {
			rel = item.ThumbPath
		}

		http.ServeFile(w, r, filepath.Join(photosDir, rel))
	}
}

func itemIDFromPath(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse id: %w", err)
	}
	return id, nil
}
