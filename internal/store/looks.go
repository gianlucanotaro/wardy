package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Look is a saved outfit: an ordered set of items, optionally captured
// straight from a suggestion (phase 8) or built by hand.
type Look struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Notes       string    `json:"notes"`
	CoverItemID NullInt64 `json:"cover_item_id"`
	Items       []Item    `json:"items"`
	CreatedAt   string    `json:"created_at"`
}

// CreateLook inserts a new look for userID with itemIDs, in order.
// cover_item_id is set to the first item. Returns ErrNotFound if any item
// id doesn't exist or doesn't belong to userID — ownership is checked as
// part of the same insert, not as a separate lookup (see insertLookItems).
func (s *Store) CreateLook(ctx context.Context, userID int64, name, notes string, itemIDs []int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// cover_item_id starts NULL and is set once itemIDs have been proven
	// to exist and belong to userID (below) — setting it straight to
	// itemIDs[0] here would let a bad id fail as a raw foreign-key
	// violation instead of the clean ErrNotFound callers expect.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO looks (user_id, name, notes) VALUES (?, ?, ?)`,
		userID, name, notes)
	if err != nil {
		return 0, fmt.Errorf("insert look: %w", err)
	}

	lookID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted id: %w", err)
	}

	if err := insertLookItems(ctx, tx, userID, lookID, itemIDs); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE looks SET cover_item_id = ? WHERE id = ?`, coverItemID(itemIDs), lookID); err != nil {
		return 0, fmt.Errorf("set look cover: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return lookID, nil
}

// LooksByUser returns userID's looks, newest first, each with its items.
func (s *Store) LooksByUser(ctx context.Context, userID int64) ([]Look, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, notes, cover_item_id, created_at
		 FROM looks WHERE user_id = ? ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query looks: %w", err)
	}
	defer rows.Close()

	var looks []Look
	for rows.Next() {
		look, err := scanLook(rows)
		if err != nil {
			return nil, err
		}
		looks = append(looks, look)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate looks: %w", err)
	}

	for i := range looks {
		items, err := itemsForLook(ctx, s.db, looks[i].ID)
		if err != nil {
			return nil, err
		}
		looks[i].Items = items
	}
	return looks, nil
}

// LookByID returns the look with id, but only if it belongs to userID —
// the ownership check, enforced here rather than in the handler.
func (s *Store) LookByID(ctx context.Context, userID, id int64) (*Look, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, notes, cover_item_id, created_at
		 FROM looks WHERE id = ? AND user_id = ?`, id, userID)

	look, err := scanLook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	items, err := itemsForLook(ctx, s.db, look.ID)
	if err != nil {
		return nil, err
	}
	look.Items = items
	return &look, nil
}

// UpdateLook renames a look and updates its notes, scoped to userID.
// Returns ErrNotFound if the look doesn't exist or belongs to a different
// user.
func (s *Store) UpdateLook(ctx context.Context, userID, id int64, name, notes string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE looks SET name = ?, notes = ? WHERE id = ? AND user_id = ?`,
		name, notes, id, userID)
	if err != nil {
		return fmt.Errorf("update look: %w", err)
	}
	return requireRowsAffected(res)
}

// ReplaceLookItems replaces a look's full item list and recomputes its
// cover, scoped to userID. Returns ErrNotFound if the look doesn't exist
// or belongs to a different user, or if any item id doesn't exist or
// doesn't belong to userID.
func (s *Store) ReplaceLookItems(ctx context.Context, userID, id int64, itemIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Clearing the cover to NULL first (rather than setting it to
	// itemIDs[0] directly) both confirms ownership via RowsAffected and
	// avoids a raw foreign-key violation if itemIDs turns out to
	// reference a nonexistent item — insertLookItems below is what turns
	// a bad id into a clean ErrNotFound.
	res, err := tx.ExecContext(ctx,
		`UPDATE looks SET cover_item_id = NULL WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("clear look cover: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM look_items WHERE look_id = ?`, id); err != nil {
		return fmt.Errorf("clear look items: %w", err)
	}
	if err := insertLookItems(ctx, tx, userID, id, itemIDs); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE looks SET cover_item_id = ? WHERE id = ?`, coverItemID(itemIDs), id); err != nil {
		return fmt.Errorf("set look cover: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DeleteLook removes a look, scoped to userID; look_items rows cascade via
// the foreign key. Returns ErrNotFound if the look doesn't exist or
// belongs to a different user.
func (s *Store) DeleteLook(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM looks WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete look: %w", err)
	}
	return requireRowsAffected(res)
}

// insertLookItems inserts look_items rows for itemIDs, in order, checking
// item ownership as part of the same insert: the SELECT only matches a row
// that both exists and belongs to userID, so zero rows affected means the
// id was bad — the caller's transaction rolls back and ErrNotFound
// surfaces, rather than silently attaching someone else's item.
func insertLookItems(ctx context.Context, tx *sql.Tx, userID, lookID int64, itemIDs []int64) error {
	for position, itemID := range itemIDs {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO look_items (look_id, item_id, position)
			SELECT ?, id, ? FROM items WHERE id = ? AND user_id = ?`,
			lookID, position, itemID, userID)
		if err != nil {
			return fmt.Errorf("insert look_items: %w", err)
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

// coverItemID picks the cover for an item list: the first item, or NULL
// if the list is empty.
func coverItemID(itemIDs []int64) any {
	if len(itemIDs) == 0 {
		return nil
	}
	return itemIDs[0]
}

// itemsForLook returns a look's items in position order. A package-level
// function (rather than a *Store method) so it can run against either the
// pool (*sql.DB) or an in-flight transaction (*sql.Tx) — both satisfy the
// same query interface.
func itemsForLook(ctx context.Context, db queryContext, lookID int64) ([]Item, error) {
	rows, err := db.QueryContext(ctx, itemColumns+
		` FROM items JOIN look_items ON look_items.item_id = items.id
		  WHERE look_items.look_id = ? ORDER BY look_items.position`, lookID)
	if err != nil {
		return nil, fmt.Errorf("query look items: %w", err)
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
		return nil, fmt.Errorf("iterate look items: %w", err)
	}
	return items, nil
}

// queryContext is satisfied by both *sql.DB and *sql.Tx.
type queryContext interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func scanLook(row rowScanner) (Look, error) {
	var l Look
	err := row.Scan(&l.ID, &l.UserID, &l.Name, &l.Notes, &l.CoverItemID, &l.CreatedAt)
	if err != nil {
		return Look{}, fmt.Errorf("scan look: %w", err)
	}
	return l, nil
}
