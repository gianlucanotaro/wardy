package store

import (
	"context"
	"fmt"
	"time"
)

// CandidateFilter narrows down items for the phase-7 rules-based outfit
// pre-filter (internal/suggest builds this from occasion/season/weather).
// It's deliberately arithmetic and cheap — no AI involved — since it's
// also the permanent fallback when AI suggestions are unavailable.
//
// Occasion, Season, and TargetWarmth are all pointers so the "nothing
// matches" diagnostic (internal/suggest) can relax exactly one constraint
// at a time by nil-ing it out and re-running the same query, rather than
// duplicating the filter logic to answer "would dropping X help?".
type CandidateFilter struct {
	Occasion             *string
	Season               *string
	TargetFormality      int
	FormalityWindow      int
	TargetWarmth         *int
	WarmthWindow         int
	ExcludeWornSinceDays int // 0 disables this filter
}

// CandidateItems returns userID's non-archived items matching filter.
func (s *Store) CandidateItems(ctx context.Context, userID int64, f CandidateFilter) ([]Item, error) {
	query := itemColumns + ` FROM items WHERE user_id = ? AND archived = 0`
	args := []any{userID}

	if f.Occasion != nil {
		query += ` AND EXISTS (SELECT 1 FROM json_each(items.occasions) WHERE value = ?)`
		args = append(args, *f.Occasion)
	}
	if f.Season != nil {
		query += ` AND EXISTS (SELECT 1 FROM json_each(items.seasons) WHERE value = ?)`
		args = append(args, *f.Season)
	}

	query += ` AND ABS(formality - ?) <= ?`
	args = append(args, f.TargetFormality, f.FormalityWindow)

	if f.TargetWarmth != nil {
		query += ` AND ABS(warmth - ?) <= ?`
		args = append(args, *f.TargetWarmth, f.WarmthWindow)
	}

	if f.ExcludeWornSinceDays > 0 {
		since := time.Now().UTC().AddDate(0, 0, -f.ExcludeWornSinceDays).Format("2006-01-02")
		query += `
			AND NOT EXISTS (
				SELECT 1 FROM wear_items wi
				JOIN wears w ON w.id = wi.wear_id
				WHERE wi.item_id = items.id AND w.user_id = ? AND w.worn_on >= ?
			)`
		args = append(args, userID, since)
	}

	query += ` ORDER BY items.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query candidate items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate items: %w", err)
	}
	return items, nil
}
