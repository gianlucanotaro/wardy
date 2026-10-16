package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/gianlucanotaro/wardy/internal/images"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/google/uuid"
)

// sampleTag marks seeded garments, so a re-run can tell which samples a
// user still has and which they deleted.
const sampleTag = "sample"

// sample is one entry of samples/garments.json: a ready-tagged garment with
// an illustration, for test accounts that need a wardrobe to work with.
type sample struct {
	Name           string   `json:"name"`
	Photo          string   `json:"photo"`
	Category       string   `json:"category"`
	Subcategory    string   `json:"subcategory"`
	ColorPrimary   string   `json:"color_primary"`
	ColorSecondary string   `json:"color_secondary"`
	Pattern        string   `json:"pattern"`
	Material       string   `json:"material"`
	Fit            string   `json:"fit"`
	Formality      [2]int   `json:"formality"`
	Warmth         [2]int   `json:"warmth"`
	Seasons        []string `json:"seasons"`
	Occasions      []string `json:"occasions"`
}

func (s sample) tags() taxonomy.Tags {
	return taxonomy.Tags{
		Category:       taxonomy.Category(s.Category),
		Subcategory:    s.Subcategory,
		ColorPrimary:   s.ColorPrimary,
		ColorSecondary: s.ColorSecondary,
		Pattern:        taxonomy.Pattern(s.Pattern),
		Material:       taxonomy.Material(s.Material),
		Fit:            taxonomy.Fit(s.Fit),
		FormalityMin:   s.Formality[0],
		FormalityMax:   s.Formality[1],
		WarmthMin:      s.Warmth[0],
		WarmthMax:      s.Warmth[1],
		Seasons:        s.Seasons,
		Occasions:      s.Occasions,
	}
}

// seedSamplesCmd implements `wardrobe seed-samples --email devtest`: gives
// that account every sample garment it doesn't have yet. Safe to run on
// every start — samples the user still has (active or archived) are left
// alone, deleted ones come back. Meant for test accounts on dev.
func seedSamplesCmd(args []string) error {
	fs := flag.NewFlagSet("seed-samples", flag.ExitOnError)
	email := fs.String("email", "", "login of the account to fill (required)")
	dir := fs.String("dir", "./samples", "directory holding garments.json and photos/")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email is required")
	}

	samples, err := loadSamples(*dir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	user, err := s.UserByEmail(ctx, *email)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no account with login %q", *email)
	} else if err != nil {
		return fmt.Errorf("look up user: %w", err)
	}

	photosDir := filepath.Join(envOr("DATA_DIR", "./data"), "photos")
	added, err := seedSamples(ctx, s, user.ID, *dir, photosDir, samples)
	if err != nil {
		return err
	}
	fmt.Printf("%s: added %d sample garments, %d already there\n", *email, added, len(samples)-added)
	return nil
}

// loadSamples reads garments.json from dir and checks every entry against
// the taxonomy and that its photo exists, so a typo fails the whole run
// instead of seeding a half-tagged item.
func loadSamples(dir string) ([]sample, error) {
	data, err := os.ReadFile(filepath.Join(dir, "garments.json"))
	if err != nil {
		return nil, fmt.Errorf("read samples: %w", err)
	}
	var samples []sample
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, fmt.Errorf("parse samples: %w", err)
	}
	names := make(map[string]bool, len(samples))
	for _, smp := range samples {
		if smp.Name == "" || names[smp.Name] {
			return nil, fmt.Errorf("sample names must be set and unique, got %q", smp.Name)
		}
		names[smp.Name] = true
		if err := smp.tags().Validate(); err != nil {
			return nil, fmt.Errorf("sample %q: %w", smp.Name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "photos", smp.Photo)); err != nil {
			return nil, fmt.Errorf("sample %q: %w", smp.Name, err)
		}
	}
	return samples, nil
}

// seedSamples adds each of samples that userID doesn't already have (by
// name, among their items tagged sampleTag), returning how many it added.
// Each goes through the same image pipeline as an upload and is saved as
// user-edited, so AI tagging never touches it.
func seedSamples(ctx context.Context, s *store.Store, userID int64, dir, photosDir string, samples []sample) (int, error) {
	have := make(map[string]bool)
	for _, archived := range []bool{false, true} {
		items, err := s.ItemsByUser(ctx, userID, archived)
		if err != nil {
			return 0, fmt.Errorf("list items: %w", err)
		}
		for _, it := range items {
			if slices.Contains(it.CustomTags, sampleTag) {
				have[it.Name] = true
			}
		}
	}

	userDir := strconv.FormatInt(userID, 10)
	if err := os.MkdirAll(filepath.Join(photosDir, userDir), 0o755); err != nil {
		return 0, fmt.Errorf("create photo dir: %w", err)
	}

	added := 0
	for _, smp := range samples {
		if have[smp.Name] {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, "photos", smp.Photo))
		if err != nil {
			return added, fmt.Errorf("read %s: %w", smp.Photo, err)
		}
		display, thumb, err := images.Process(raw)
		if err != nil {
			return added, fmt.Errorf("process %s: %w", smp.Photo, err)
		}
		id := uuid.NewString()
		photoRel := filepath.Join(userDir, id+".jpg")
		thumbRel := filepath.Join(userDir, id+"_thumb.jpg")
		if err := os.WriteFile(filepath.Join(photosDir, photoRel), display, 0o644); err != nil {
			return added, fmt.Errorf("write photo: %w", err)
		}
		if err := os.WriteFile(filepath.Join(photosDir, thumbRel), thumb, 0o644); err != nil {
			return added, fmt.Errorf("write thumb: %w", err)
		}

		itemID, err := s.CreateItem(ctx, userID, photoRel, thumbRel)
		if err != nil {
			return added, fmt.Errorf("create %q: %w", smp.Name, err)
		}
		t := smp.tags()
		if err := s.UpdateItemTags(ctx, userID, itemID, store.ItemTags{
			Category: string(t.Category), Subcategory: t.Subcategory,
			ColorPrimary: t.ColorPrimary, ColorSecondary: t.ColorSecondary,
			Pattern: string(t.Pattern), Material: string(t.Material), Fit: string(t.Fit),
			FormalityMin: t.FormalityMin, FormalityMax: t.FormalityMax,
			WarmthMin: t.WarmthMin, WarmthMax: t.WarmthMax,
			Seasons: t.Seasons, Occasions: t.Occasions, CustomTags: []string{sampleTag},
		}); err != nil {
			return added, fmt.Errorf("tag %q: %w", smp.Name, err)
		}
		if err := s.UpdateItemName(ctx, userID, itemID, smp.Name); err != nil {
			return added, fmt.Errorf("name %q: %w", smp.Name, err)
		}
		added++
	}
	return added, nil
}
