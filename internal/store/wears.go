package store

import (
	"context"
	"fmt"
	"time"
)

// LogWear records that itemIDs were worn on wornOn (date only; time of day
// is discarded) for userID. A minimal building block for phase 10's wear
// logging feature — used already by the outfit pre-filter's "not worn
// recently" exclusion (internal/suggest).
func (s *Store) LogWear(ctx context.Context, userID int64, itemIDs []int64, wornOn time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO wears (user_id, worn_on) VALUES (?, ?)`,
		userID, wornOn.UTC().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("insert wear: %w", err)
	}

	wearID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get inserted wear id: %w", err)
	}

	for _, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO wear_items (wear_id, item_id) VALUES (?, ?)`, wearID, itemID); err != nil {
			return fmt.Errorf("insert wear_items: %w", err)
		}
	}

	return tx.Commit()
}
