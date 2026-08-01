package http

import (
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
	"github.com/gianlucanotaro/wardy/internal/store"
)

// maxUploadBytes caps the raw request body, not just the decoded image —
// oversized uploads get rejected before any image processing happens.
const maxUploadBytes = 15 << 20 // 15 MB

// uploadItemHandler accepts a multipart "photo" field, runs it through the
// image pipeline (orientation fix, resize, EXIF strip), stores both
// derivatives on disk, and creates the item row.
func uploadItemHandler(s *store.Store, photosDir string) http.HandlerFunc {
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

		item, err := s.ItemByID(r.Context(), user.ID, itemID)
		if err != nil {
			slog.Error("upload: fetch created item", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(item)
	}
}

func listItemsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		items, err := s.ItemsByUser(r.Context(), user.ID)
		if err != nil {
			slog.Error("list items", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []store.Item{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
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
