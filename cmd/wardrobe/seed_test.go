package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// The checked-in samples must stay valid: every entry passes the taxonomy
// and has its photo, and together they cover every category.
func TestLoadSamplesRepoManifest(t *testing.T) {
	samples, err := loadSamples("../../samples")
	if err != nil {
		t.Fatalf("loadSamples: %v", err)
	}
	categories := make(map[string]int)
	for _, smp := range samples {
		categories[smp.Category]++
	}
	for _, c := range []string{"top", "bottom", "dress", "outerwear", "shoes", "bag", "accessory"} {
		if categories[c] < 2 {
			t.Errorf("want at least 2 %s samples, got %d", c, categories[c])
		}
	}
}

// Seeding twice adds nothing the second time; a deleted sample comes back,
// an archived one stays archived and isn't duplicated.
func TestSeedSamplesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	userID, err := s.CreateUser(ctx, "devtest", "x", "devtest")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	samples, err := loadSamples("../../samples")
	if err != nil {
		t.Fatalf("loadSamples: %v", err)
	}
	photos := t.TempDir()

	if added, err := seedSamples(ctx, s, userID, "../../samples", photos, samples); err != nil || added != len(samples) {
		t.Fatalf("first seed: added %d, err %v; want %d", added, err, len(samples))
	}
	if added, err := seedSamples(ctx, s, userID, "../../samples", photos, samples); err != nil || added != 0 {
		t.Fatalf("second seed: added %d, err %v; want 0", added, err)
	}

	items, err := s.ItemsByUser(ctx, userID, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	first := items[0].Item
	if first.Name == "" || first.UserEditedAt.String == "" {
		t.Errorf("seeded item should be named and user-edited: %+v", first)
	}
	if _, _, err := s.DeleteItem(ctx, userID, items[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.ArchiveItem(ctx, userID, items[1].ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if added, err := seedSamples(ctx, s, userID, "../../samples", photos, samples); err != nil || added != 1 {
		t.Fatalf("after delete+archive: added %d, err %v; want 1", added, err)
	}
}
