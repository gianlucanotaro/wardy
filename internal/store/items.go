package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Item is a wardrobe item. Tagging fields (category, subcategory, colors,
// pattern, material, formality, warmth, fit, seasons, occasions) are
// populated by manual tagging (phase 4) and AI tagging (phase 5); on
// upload they start blank/default.
type Item struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Name           string     `json:"name"`
	Category       string     `json:"category"`
	Subcategory    string     `json:"subcategory"`
	ColorPrimary   string     `json:"color_primary"`
	ColorSecondary string     `json:"color_secondary"`
	Pattern        string     `json:"pattern"`
	Material       string     `json:"material"`
	Formality      int        `json:"formality"`
	Warmth         int        `json:"warmth"`
	Fit            string     `json:"fit"`
	Brand          string     `json:"brand"`
	Notes          string     `json:"notes"`
	Seasons        string     `json:"seasons"`   // JSON array, e.g. `["summer"]`
	Occasions      string     `json:"occasions"` // JSON array
	PhotoPath      string     `json:"photo_path"`
	ThumbPath      string     `json:"thumb_path"`
	AITaggedAt     NullString `json:"ai_tagged_at"`
	UserEditedAt   NullString `json:"user_edited_at"`
	Archived       bool       `json:"archived"`
	CreatedAt      string     `json:"created_at"`
}

// CreateItem inserts a new item for userID with just its photo paths;
// every taxonomy field is left at its schema default until tagging fills
// it in.
func (s *Store) CreateItem(ctx context.Context, userID int64, photoPath, thumbPath string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (user_id, category, photo_path, thumb_path) VALUES (?, '', ?, ?)`,
		userID, photoPath, thumbPath)
	if err != nil {
		return 0, fmt.Errorf("insert item: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted id: %w", err)
	}
	return id, nil
}

// ItemsByUser returns userID's non-archived items, newest first.
func (s *Store) ItemsByUser(ctx context.Context, userID int64) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, itemColumns+
		` FROM items WHERE user_id = ? AND archived = 0 ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
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
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	return items, nil
}

// ItemByID returns the item with id, but only if it belongs to userID —
// this is the ownership check, enforced here rather than in the handler so
// a missed check elsewhere can't leak another user's item.
func (s *Store) ItemByID(ctx context.Context, userID, id int64) (*Item, error) {
	row := s.db.QueryRowContext(ctx, itemColumns+
		` FROM items WHERE id = ? AND user_id = ?`,
		id, userID)

	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItemName renames an item, scoped to userID. Returns ErrNotFound if
// the item doesn't exist or belongs to a different user.
func (s *Store) UpdateItemName(ctx context.Context, userID, id int64, name string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE items
		SET name = ?, user_edited_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ? AND user_id = ?`,
		name, id, userID)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return requireRowsAffected(res)
}

// DeleteItem removes an item's row, scoped to userID, and returns its photo
// paths so the caller can clean up the files on disk. Returns ErrNotFound
// if the item doesn't exist or belongs to a different user.
func (s *Store) DeleteItem(ctx context.Context, userID, id int64) (photoPath, thumbPath string, err error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT photo_path, thumb_path FROM items WHERE id = ? AND user_id = ?`,
		id, userID)
	if err := row.Scan(&photoPath, &thumbPath); errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	} else if err != nil {
		return "", "", fmt.Errorf("look up item: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return "", "", fmt.Errorf("delete item: %w", err)
	}

	return photoPath, thumbPath, nil
}

const itemColumns = `SELECT id, user_id, name, category, subcategory, color_primary,
	color_secondary, pattern, material, formality, warmth, fit, brand, notes,
	seasons, occasions, photo_path, thumb_path, ai_tagged_at, user_edited_at,
	archived, created_at`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var it Item
	err := row.Scan(
		&it.ID, &it.UserID, &it.Name, &it.Category, &it.Subcategory, &it.ColorPrimary,
		&it.ColorSecondary, &it.Pattern, &it.Material, &it.Formality, &it.Warmth, &it.Fit,
		&it.Brand, &it.Notes, &it.Seasons, &it.Occasions, &it.PhotoPath, &it.ThumbPath,
		&it.AITaggedAt, &it.UserEditedAt, &it.Archived, &it.CreatedAt,
	)
	if err != nil {
		return Item{}, fmt.Errorf("scan item: %w", err)
	}
	return it, nil
}

func requireRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
