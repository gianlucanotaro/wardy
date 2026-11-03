package store

import (
	"context"
	"fmt"
)

// ItemStyleLists returns the style list of each of userID's active
// (unarchived) items, and of each item in userID's wears on or after
// wornSince (YYYY-MM-DD) — once per wear, so an item worn twice counts
// twice. Items without styles come back as empty lists. Feeds the style
// radar's owned and worn layers.
func (s *Store) ItemStyleLists(ctx context.Context, userID int64, wornSince string) (owned, worn [][]string, err error) {
	owned, err = s.styleLists(ctx,
		`SELECT styles FROM items WHERE user_id = ? AND archived = 0`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("owned styles: %w", err)
	}
	worn, err = s.styleLists(ctx, `
		SELECT items.styles FROM wears
		JOIN wear_items ON wear_items.wear_id = wears.id
		JOIN items ON items.id = wear_items.item_id
		WHERE wears.user_id = ? AND items.user_id = ? AND wears.worn_on >= ?`,
		userID, userID, wornSince)
	if err != nil {
		return nil, nil, fmt.Errorf("worn styles: %w", err)
	}
	return owned, worn, nil
}

func (s *Store) styleLists(ctx context.Context, query string, args ...any) ([][]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lists [][]string
	for rows.Next() {
		var styles StringList
		if err := rows.Scan(&styles); err != nil {
			return nil, fmt.Errorf("scan styles: %w", err)
		}
		lists = append(lists, styles)
	}
	return lists, rows.Err()
}
