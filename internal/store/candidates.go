package store

import (
	"context"
	"fmt"
	"time"
)

// CandidateFilter narrows down items for the phase-7 rules-based outfit
// pre-filter (internal/suggest builds this from the occasion). It's
// deliberately arithmetic and cheap — no AI involved — since it's also the
// permanent fallback when AI suggestions are unavailable.
//
// Only the hard limits live here. Weather fit depends on the item's
// category and how outfits get layered, and occasion/season tags are
// preferences rather than filters, so internal/suggest handles both. The
// "nothing matches" diagnostic there relaxes one field at a time and
// re-runs this same query.
type CandidateFilter struct {
	FormalityMin         int
	FormalityMax         int
	ExcludeWornSinceDays int // 0 disables this filter
}

// CandidateItems returns userID's non-archived items matching filter. An
// item qualifies when its own formality range overlaps the filter's.
func (s *Store) CandidateItems(ctx context.Context, userID int64, f CandidateFilter) ([]Item, error) {
	query := itemColumns + ` FROM items WHERE user_id = ? AND archived = 0 AND formality_min <= ? AND formality_max >= ?`
	args := []any{userID, f.FormalityMax, f.FormalityMin}

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
