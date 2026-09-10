package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Wear is one instance of wearing a set of items on a date — optionally
// tied to a saved look, or ad-hoc items chosen for the occasion.
type Wear struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	LookID         NullInt64 `json:"look_id"`
	WornOn         string    `json:"worn_on"`
	Occasion       string    `json:"occasion"`
	WeatherSummary string    `json:"weather_summary"`
	Notes          string    `json:"notes"`
	Items          []Item    `json:"items"`
	CreatedAt      string    `json:"created_at"`
}

// LogWear records that itemIDs were worn on wornOn (date only; time of day
// is discarded) for userID, optionally attributed to lookID. Returns
// ErrNotFound if lookID doesn't belong to userID, or if any item id
// doesn't exist or doesn't belong to userID — ownership is checked as
// part of the same inserts, not as a separate lookup (see
// insertWearItems), the same structural guarantee internal/store/looks.go
// uses.
func (s *Store) LogWear(ctx context.Context, userID int64, lookID *int64, itemIDs []int64, wornOn time.Time, occasion, notes string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if lookID != nil {
		var exists bool
		err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM looks WHERE id = ? AND user_id = ?)`, *lookID, userID).Scan(&exists)
		if err != nil {
			return 0, fmt.Errorf("check look ownership: %w", err)
		}
		if !exists {
			return 0, ErrNotFound
		}
	}

	// lookID stays nil (SQL NULL) until here, deliberately — writing it
	// straight into the INSERT below without the ownership check above
	// would let a nonexistent look id fail as a raw foreign-key
	// violation instead of the ErrNotFound callers expect.
	var lookIDArg any
	if lookID != nil {
		lookIDArg = *lookID
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO wears (user_id, look_id, worn_on, occasion, notes) VALUES (?, ?, ?, ?, ?)`,
		userID, lookIDArg, wornOn.UTC().Format("2006-01-02"), occasion, notes)
	if err != nil {
		return 0, fmt.Errorf("insert wear: %w", err)
	}

	wearID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted wear id: %w", err)
	}

	if err := insertWearItems(ctx, tx, userID, wearID, itemIDs); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return wearID, nil
}

// WearsByUser returns userID's wears, most recently worn first, each with
// its items.
func (s *Store) WearsByUser(ctx context.Context, userID int64) ([]Wear, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, look_id, worn_on, occasion, weather_summary, notes, created_at
		 FROM wears WHERE user_id = ? ORDER BY worn_on DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query wears: %w", err)
	}
	defer rows.Close()

	var wears []Wear
	for rows.Next() {
		wear, err := scanWear(rows)
		if err != nil {
			return nil, err
		}
		wears = append(wears, wear)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wears: %w", err)
	}

	for i := range wears {
		items, err := itemsForWear(ctx, s.db, wears[i].ID)
		if err != nil {
			return nil, err
		}
		wears[i].Items = items
	}
	return wears, nil
}

// WearByID returns the wear with id, but only if it belongs to userID.
func (s *Store) WearByID(ctx context.Context, userID, id int64) (*Wear, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, look_id, worn_on, occasion, weather_summary, notes, created_at
		 FROM wears WHERE id = ? AND user_id = ?`, id, userID)

	wear, err := scanWear(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	items, err := itemsForWear(ctx, s.db, wear.ID)
	if err != nil {
		return nil, err
	}
	wear.Items = items
	return &wear, nil
}

// DeleteWear removes a wear log entry, scoped to userID; wear_items rows
// cascade via the foreign key. The look it referenced (if any) is
// untouched. Returns ErrNotFound if the wear doesn't exist or belongs to
// a different user.
func (s *Store) DeleteWear(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM wears WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete wear: %w", err)
	}
	return requireRowsAffected(res)
}

// ItemLastWorn pairs an item with the most recent date it was worn, for
// the "not worn in a while" view — LastWornOn is invalid for an item
// that's never been logged as worn.
type ItemLastWorn struct {
	Item
	LastWornOn NullString `json:"last_worn_on"`
}

// ItemsByLastWorn returns userID's non-archived items ordered by staleness
// — least recently worn first, with never-worn items first of all (SQLite
// sorts NULL before any value in ascending order, so no separate case is
// needed for "never worn").
func (s *Store) ItemsByLastWorn(ctx context.Context, userID int64) ([]ItemLastWorn, error) {
	// itemColumns' bare column names (id, user_id, created_at) collide
	// with columns of the same name on the joined wears table, so this
	// query needs its own items.-qualified list rather than reusing it.
	rows, err := s.db.QueryContext(ctx, `SELECT items.id, items.user_id, items.name, items.category, items.subcategory,
		items.color_primary, items.color_secondary, items.pattern, items.material, items.formality, items.warmth,
		items.fit, items.brand, items.notes, items.seasons, items.occasions, items.custom_tags, items.photo_path, items.thumb_path,
		items.ai_tagged_at, items.user_edited_at, items.archived, items.created_at, MAX(w.worn_on)
		FROM items
		LEFT JOIN wear_items wi ON wi.item_id = items.id
		LEFT JOIN wears w ON w.id = wi.wear_id
		WHERE items.user_id = ? AND items.archived = 0
		GROUP BY items.id
		ORDER BY MAX(w.worn_on) ASC, items.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query items by last worn: %w", err)
	}
	defer rows.Close()

	var items []ItemLastWorn
	for rows.Next() {
		item, err := scanItemWithLastWorn(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items by last worn: %w", err)
	}
	return items, nil
}

// scanItemWithLastWorn scans a row from ItemsByLastWorn's query: every
// itemColumns field plus the trailing MAX(w.worn_on). Not built on top of
// scanItem — database/sql.Scan requires the destination count to match
// the column count exactly, so the extra column has to be part of the
// same Scan call.
func scanItemWithLastWorn(row rowScanner) (ItemLastWorn, error) {
	var it ItemLastWorn
	err := row.Scan(
		&it.ID, &it.UserID, &it.Name, &it.Category, &it.Subcategory, &it.ColorPrimary,
		&it.ColorSecondary, &it.Pattern, &it.Material, &it.Formality, &it.Warmth, &it.Fit,
		&it.Brand, &it.Notes, &it.Seasons, &it.Occasions, &it.CustomTags, &it.PhotoPath, &it.ThumbPath,
		&it.AITaggedAt, &it.UserEditedAt, &it.Archived, &it.CreatedAt,
		&it.LastWornOn,
	)
	if err != nil {
		return ItemLastWorn{}, fmt.Errorf("scan item with last worn: %w", err)
	}
	return it, nil
}

// insertWearItems inserts wear_items rows for itemIDs, checking item
// ownership as part of the same insert: the SELECT only matches a row
// that both exists and belongs to userID, so zero rows affected means the
// id was bad — the caller's transaction rolls back and ErrNotFound
// surfaces. Same pattern as internal/store/looks.go's insertLookItems.
func insertWearItems(ctx context.Context, tx *sql.Tx, userID, wearID int64, itemIDs []int64) error {
	for _, itemID := range itemIDs {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO wear_items (wear_id, item_id)
			SELECT ?, id FROM items WHERE id = ? AND user_id = ?`,
			wearID, itemID, userID)
		if err != nil {
			return fmt.Errorf("insert wear_items: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// itemsForWear returns a wear's items, ordered by id for determinism
// (wear_items carries no ordering column — unlike a look, a wear isn't a
// sequenced outfit, just the set of items worn).
func itemsForWear(ctx context.Context, db queryContext, wearID int64) ([]Item, error) {
	rows, err := db.QueryContext(ctx, itemColumns+
		` FROM items JOIN wear_items ON wear_items.item_id = items.id
		  WHERE wear_items.wear_id = ? ORDER BY items.id`, wearID)
	if err != nil {
		return nil, fmt.Errorf("query wear items: %w", err)
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
		return nil, fmt.Errorf("iterate wear items: %w", err)
	}
	return items, nil
}

func scanWear(row rowScanner) (Wear, error) {
	var w Wear
	err := row.Scan(&w.ID, &w.UserID, &w.LookID, &w.WornOn, &w.Occasion, &w.WeatherSummary, &w.Notes, &w.CreatedAt)
	if err != nil {
		return Wear{}, fmt.Errorf("scan wear: %w", err)
	}
	return w, nil
}
