package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// createLookHandler saves a set of items as a look — used both for
// hand-built looks and for "save this suggestion" from /api/suggest
// results (phase 8). item_ids must be non-empty; every id must belong to
// the caller.
func createLookHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			Name    string  `json:"name"`
			Notes   string  `json:"notes"`
			ItemIDs []int64 `json:"item_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(req.ItemIDs) == 0 {
			http.Error(w, "a look needs at least one item", http.StatusBadRequest)
			return
		}

		lookID, err := s.CreateLook(r.Context(), user.ID, req.Name, req.Notes, req.ItemIDs)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "one or more items not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			slog.Error("create look", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		look, err := s.LookByID(r.Context(), user.ID, lookID)
		if err != nil {
			slog.Error("create look: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(look)
	}
}

func listLooksHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		looks, err := s.LooksByUser(r.Context(), user.ID)
		if err != nil {
			slog.Error("list looks", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if looks == nil {
			looks = []store.Look{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(looks)
	}
}

func getLookHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid look id", http.StatusBadRequest)
			return
		}

		look, err := s.LookByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("get look", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(look)
	}
}

// updateLookHandler applies a partial edit: name and/or the full item
// list, whichever fields are present. Replacing item_ids with an empty
// list is rejected the same way creation is — a look always has at least
// one item; deleting the last one means deleting the look.
func updateLookHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid look id", http.StatusBadRequest)
			return
		}

		var req struct {
			Name    *string  `json:"name"`
			ItemIDs *[]int64 `json:"item_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.ItemIDs != nil && len(*req.ItemIDs) == 0 {
			http.Error(w, "a look needs at least one item", http.StatusBadRequest)
			return
		}

		// Fetch first, regardless of which fields are present, so a
		// missing/not-owned look is always a 404 — once past this point,
		// ErrNotFound from ReplaceLookItems can only mean a bad item id.
		current, err := s.LookByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("update look: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if req.Name != nil {
			if err := s.UpdateLook(r.Context(), user.ID, id, *req.Name, current.Notes); err != nil {
				slog.Error("update look", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		if req.ItemIDs != nil {
			err := s.ReplaceLookItems(r.Context(), user.ID, id, *req.ItemIDs)
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "one or more items not found", http.StatusBadRequest)
				return
			}
			if err != nil {
				slog.Error("update look items", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		look, err := s.LookByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("update look: refetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(look)
	}
}

func deleteLookHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid look id", http.StatusBadRequest)
			return
		}

		if err := s.DeleteLook(r.Context(), user.ID, id); errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			slog.Error("delete look", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
