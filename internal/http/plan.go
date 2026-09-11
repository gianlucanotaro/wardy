package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// planDateLayout is date-only, matching plan_days.plan_date and the same
// layout createWearHandler uses for worn_on.
const planDateLayout = "2006-01-02"

// planRangeMaxDays caps a single /api/plan request — the weekly planner
// only ever asks for 7 days; this just guards against an unbounded range.
const planRangeMaxDays = 31

// planHandler returns userID's plan for every date from start to end
// (inclusive, both required query params), one PlanDay per date whether or
// not it's been assigned.
func planHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		start, err := time.Parse(planDateLayout, r.URL.Query().Get("start"))
		if err != nil {
			http.Error(w, "start must be a date in YYYY-MM-DD format", http.StatusBadRequest)
			return
		}
		end, err := time.Parse(planDateLayout, r.URL.Query().Get("end"))
		if err != nil {
			http.Error(w, "end must be a date in YYYY-MM-DD format", http.StatusBadRequest)
			return
		}
		if end.Before(start) {
			http.Error(w, "end must not be before start", http.StatusBadRequest)
			return
		}
		if end.Sub(start) > planRangeMaxDays*24*time.Hour {
			http.Error(w, "range too large", http.StatusBadRequest)
			return
		}

		days, err := s.PlanDaysInRange(r.Context(), user.ID, r.URL.Query().Get("start"), r.URL.Query().Get("end"))
		if err != nil {
			slog.Error("get plan", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(days)
	}
}

// setPlanDayHandler assigns (or clears, when look_id is null) the look
// planned for a single date.
func setPlanDayHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		date := r.PathValue("date")
		if _, err := time.Parse(planDateLayout, date); err != nil {
			http.Error(w, "invalid date", http.StatusBadRequest)
			return
		}

		var req struct {
			LookID *int64 `json:"look_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if err := s.UpsertPlanDay(r.Context(), user.ID, date, req.LookID); errors.Is(err, store.ErrNotFound) {
			http.Error(w, "look not found", http.StatusBadRequest)
			return
		} else if err != nil {
			slog.Error("set plan day", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		days, err := s.PlanDaysInRange(r.Context(), user.ID, date, date)
		if err != nil || len(days) != 1 {
			slog.Error("set plan day: refetch", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(days[0])
	}
}
