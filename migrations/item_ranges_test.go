package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// 0006 turns single formality/warmth values into ranges. Run it against
// real pre-0006 rows and check every category spreads the way the outfit
// rules before it treated that category, so existing wardrobes suggest the
// same outfits after the upgrade.
func TestItemRangesMigration(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (filename TEXT PRIMARY KEY, applied_at TEXT)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, name := range []string{"0001_schema.sql", "0002_user_location.sql", "0003_usage_user_id.sql", "0004_custom_tags.sql", "0005_plan.sql"} {
		if err := apply(ctx, db, name); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, email, password_hash, display_name) VALUES (1, 'me@example.com', 'x', 'Me')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	cases := []struct {
		category               string
		formality, warmth      int
		fMin, fMax, wMin, wMax int
	}{
		{"top", 4, 2, 4, 4, 1, 5},       // layered on any colder day, one step warmer at most
		{"top", 2, 1, 2, 2, 1, 5},       // clamped at 1
		{"top", 3, 5, 3, 3, 4, 5},       // a heavy knit still has no place in a heatwave
		{"outerwear", 3, 5, 3, 3, 4, 5}, // one step either side, clamped at 5
		{"outerwear", 2, 2, 2, 2, 1, 3}, // one step either side
		{"bottom", 2, 1, 2, 2, 1, 3},    // shorts: one warmer, two cooler
		{"bottom", 2, 4, 2, 2, 3, 5},    // jeans
		{"dress", 3, 3, 3, 3, 2, 5},     // same as bottoms
		{"shoes", 4, 4, 4, 4, 3, 5},     // same as bottoms
		{"bag", 3, 3, 3, 3, 1, 5},       // any weather
		{"accessory", 2, 4, 2, 2, 1, 5}, // any weather
	}
	for i, c := range cases {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO items (id, user_id, category, formality, warmth, photo_path, thumb_path)
			VALUES (?, 1, ?, ?, ?, 'p.jpg', 't.jpg')`, i+1, c.category, c.formality, c.warmth); err != nil {
			t.Fatalf("insert item: %v", err)
		}
	}

	if err := apply(ctx, db, "0006_item_ranges.sql"); err != nil {
		t.Fatalf("apply 0006: %v", err)
	}

	for i, c := range cases {
		var fMin, fMax, wMin, wMax int
		err := db.QueryRowContext(ctx,
			`SELECT formality_min, formality_max, warmth_min, warmth_max FROM items WHERE id = ?`, i+1,
		).Scan(&fMin, &fMax, &wMin, &wMax)
		if err != nil {
			t.Fatalf("select item %d: %v", i+1, err)
		}
		if fMin != c.fMin || fMax != c.fMax || wMin != c.wMin || wMax != c.wMax {
			t.Errorf("%s formality %d warmth %d -> formality %d-%d warmth %d-%d, want formality %d-%d warmth %d-%d",
				c.category, c.formality, c.warmth, fMin, fMax, wMin, wMax, c.fMin, c.fMax, c.wMin, c.wMax)
		}
	}
}
