package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// wornOnLayout is date-only, matching store.Wear.WornOn / the wears.worn_on
// column — time of day is deliberately discarded.
const wornOnLayout = "2006-01-02"

// createWearHandler logs a wear, either from a saved look (look_id — the
// item list is derived from the look, ownership-checked here so a bad
// look_id gets a clean 404) or ad-hoc items (item_ids).
func createWearHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			LookID   *int64  `json:"look_id"`
			ItemIDs  []int64 `json:"item_ids"`
			WornOn   string  `json:"worn_on"`
			Occasion string  `json:"occasion"`
			Notes    string  `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		wornOn, err := time.Parse(wornOnLayout, req.WornOn)
		if err != nil {
			http.Error(w, "worn_on must be a date in YYYY-MM-DD format", http.StatusBadRequest)
			return
		}

		if req.Occasion != "" && !taxonomy.Occasion(req.Occasion).Valid() {
			http.Error(w, "invalid occasion", http.StatusBadRequest)
			return
		}

		itemIDs := req.ItemIDs
		if req.LookID != nil {
			look, err := s.LookByID(r.Context(), user.ID, *req.LookID)
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "look not found", http.StatusNotFound)
				return
			}
			if err != nil {
				slog.Error("log wear: fetch look", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			itemIDs = make([]int64, len(look.Items))
			for i, item := range look.Items {
				itemIDs[i] = item.ID
			}
		} else if len(itemIDs) == 0 {
			http.Error(w, "provide either look_id or item_ids", http.StatusBadRequest)
			return
		}

		wearID, err := s.LogWear(r.Context(), user.ID, req.LookID, itemIDs, wornOn, req.Occasion, req.Notes)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "one or more items not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			slog.Error("log wear", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		wear, err := s.WearByID(r.Context(), user.ID, wearID)
		if err != nil {
			slog.Error("log wear: fetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(wear)
	}
}

func listWearsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		wears, err := s.WearsByUser(r.Context(), user.ID)
		if err != nil {
			slog.Error("list wears", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if wears == nil {
			wears = []store.Wear{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wears)
	}
}

func getWearHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid wear id", http.StatusBadRequest)
			return
		}

		wear, err := s.WearByID(r.Context(), user.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("get wear", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wear)
	}
}

func deleteWearHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := itemIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid wear id", http.StatusBadRequest)
			return
		}

		if err := s.DeleteWear(r.Context(), user.ID, id); errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			slog.Error("delete wear", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type historyResponse struct {
	Wears []store.Wear         `json:"wears"`
	Items []store.ItemLastWorn `json:"items"`
}

// historyHandler serves everything the /history screen needs in one call:
// the wear log and every item ordered by staleness (least-recently-worn,
// including never-worn, first) — docs/frontend.md's "wear log, 'not worn
// in a while'".
func historyHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		wears, err := s.WearsByUser(r.Context(), user.ID)
		if err != nil {
			slog.Error("history: list wears", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if wears == nil {
			wears = []store.Wear{}
		}

		items, err := s.ItemsByLastWorn(r.Context(), user.ID)
		if err != nil {
			slog.Error("history: items by last worn", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []store.ItemLastWorn{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(historyResponse{Wears: wears, Items: items})
	}
}
