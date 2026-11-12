package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PlanDay is a single calendar date's planned look — the weekly planner's
// unit. Look is nil for an unplanned date. Wears is what was logged as
// worn that date, oldest first, and never nil (an empty list encodes as []).
type PlanDay struct {
	Date  string `json:"date"`
	Look  *Look  `json:"look"`
	Wears []Wear `json:"wears"`
}

// UpsertPlanDay assigns lookID to date for userID, or clears it when lookID
// is nil. Returns ErrNotFound if lookID doesn't belong to userID — checked
// before the write lands, the same structural guarantee
// internal/store/wears.go's LogWear uses for its own optional look_id.
func (s *Store) UpsertPlanDay(ctx context.Context, userID int64, date string, lookID *int64) error {
	if lookID != nil {
		var exists bool
		err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM looks WHERE id = ? AND user_id = ?)`, *lookID, userID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check look ownership: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
	}

	var lookIDArg any
	if lookID != nil {
		lookIDArg = *lookID
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plan_days (user_id, plan_date, look_id) VALUES (?, ?, ?)
		ON CONFLICT(user_id, plan_date) DO UPDATE SET look_id = excluded.look_id`,
		userID, date, lookIDArg)
	if err != nil {
		return fmt.Errorf("upsert plan day: %w", err)
	}
	return nil
}

// PlanDaysInRange returns one PlanDay per date from start to end (inclusive,
// both "YYYY-MM-DD"), in order — a date with no row, or whose look_id was
// cleared by ON DELETE SET NULL, comes back with Look == nil rather than
// being omitted, so callers always get a fixed-length week.
func (s *Store) PlanDaysInRange(ctx context.Context, userID int64, start, end string) ([]PlanDay, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT plan_date, look_id FROM plan_days
		 WHERE user_id = ? AND plan_date BETWEEN ? AND ?`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query plan days: %w", err)
	}
	defer rows.Close()

	lookIDs := make(map[string]int64)
	for rows.Next() {
		var date string
		var lookID NullInt64
		if err := rows.Scan(&date, &lookID); err != nil {
			return nil, fmt.Errorf("scan plan day: %w", err)
		}
		if lookID.Valid {
			lookIDs[date] = lookID.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan days: %w", err)
	}

	dates, err := datesInRange(start, end)
	if err != nil {
		return nil, err
	}

	wears, err := s.wearsInRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	days := make([]PlanDay, len(dates))
	for i, date := range dates {
		days[i] = PlanDay{Date: date, Wears: wears[date]}
		if days[i].Wears == nil {
			days[i].Wears = []Wear{}
		}
		if lookID, ok := lookIDs[date]; ok {
			look, err := s.LookByID(ctx, userID, lookID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return nil, err
			}
			days[i].Look = look
		}
	}
	return days, nil
}

// datesInRange expands start..end (inclusive, "YYYY-MM-DD") into individual
// dates. Capped at 31 days — the weekly planner only ever asks for 7, this
// just guards against an unbounded range being requested.
func datesInRange(start, end string) ([]string, error) {
	startT, err := parseDate(start)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	endT, err := parseDate(end)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}
	if endT.Before(startT) {
		return nil, fmt.Errorf("end date before start date")
	}

	var dates []string
	for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
		if len(dates) > 31 {
			return nil, fmt.Errorf("range too large")
		}
	}
	return dates, nil
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
