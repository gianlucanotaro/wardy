package store

import (
	"context"
	"testing"
)

func TestUpdateItemAITagsAppliesWhenNotUserEdited(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	applied, err := s.UpdateItemAITags(ctx, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
	})
	if err != nil {
		t.Fatalf("UpdateItemAITags: %v", err)
	}
	if !applied {
		t.Fatal("expected AI tags to be applied to a never-edited item")
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if item.Category != "top" || !item.AITaggedAt.Valid {
		t.Errorf("AI tags not applied: %+v", item)
	}
}

func TestUpdateItemAITagsSkipsUserEditedItem(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	// Simulate a manual edit happening before the AI job runs.
	if err := s.UpdateItemTags(ctx, userID, itemID, ItemTags{
		Category: "bottom", Subcategory: "jeans", ColorPrimary: "navy",
		Pattern: "solid", Material: "denim", Fit: "slim",
		Formality: 2, Warmth: 3, Seasons: []string{"autumn"}, Occasions: []string{"casual"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	applied, err := s.UpdateItemAITags(ctx, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
	})
	if err != nil {
		t.Fatalf("UpdateItemAITags: %v", err)
	}
	if applied {
		t.Fatal("AI tags should not overwrite a user-edited item")
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if item.Category != "bottom" {
		t.Errorf("user's manual tags were overwritten: %+v", item)
	}
	if item.AITaggedAt.Valid {
		t.Errorf("ai_tagged_at should remain unset when the AI update was skipped")
	}
}

func TestUpdateItemTagsRoundTripsCustomTags(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	if err := s.UpdateItemTags(ctx, userID, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"favorite", "gift from mom"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if len(item.CustomTags) != 2 || item.CustomTags[0] != "favorite" || item.CustomTags[1] != "gift from mom" {
		t.Errorf("CustomTags = %+v, want [favorite, gift from mom]", item.CustomTags)
	}
}

func TestUpdateItemAITagsSetsCustomTagsOnlyWhenNotUserEdited(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	// AI tagging (including its proposed custom tags) applies to a
	// never-edited item.
	if _, err := s.UpdateItemAITags(ctx, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"vintage", "distressed"},
	}); err != nil {
		t.Fatalf("UpdateItemAITags: %v", err)
	}

	item, err := s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if len(item.CustomTags) != 2 || item.CustomTags[0] != "vintage" {
		t.Fatalf("AI-proposed custom tags not applied: %+v", item.CustomTags)
	}

	// The user removes one of the AI's suggestions — this must count as
	// a user edit protecting the item (including its custom tags) from
	// a later AI re-run, exactly like every other tag field.
	if err := s.UpdateItemTags(ctx, userID, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"vintage"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	applied, err := s.UpdateItemAITags(ctx, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: 2, Warmth: 3, Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"vintage", "distressed", "logo-print"},
	})
	if err != nil {
		t.Fatalf("UpdateItemAITags: %v", err)
	}
	if applied {
		t.Fatal("AI re-run should not overwrite custom tags on a user-edited item")
	}

	item, err = s.ItemByID(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("ItemByID: %v", err)
	}
	if len(item.CustomTags) != 1 || item.CustomTags[0] != "vintage" {
		t.Errorf("user's custom tag edit was overwritten: %+v", item.CustomTags)
	}
}

func TestValidateCustomTags(t *testing.T) {
	longTag := ""
	for i := 0; i < 40; i++ {
		longTag += "a"
	}

	got := validateCustomTags([]string{
		"  favorite  ", "", "   ", "favorite", "duplicate", "duplicate", longTag,
	})

	want := []string{"favorite", "duplicate", longTag[:30]}
	if len(got) != len(want) {
		t.Fatalf("validateCustomTags = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateCustomTagsCapsCount(t *testing.T) {
	tags := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		tags = append(tags, string(rune('a'+i)))
	}

	got := validateCustomTags(tags)
	if len(got) != maxCustomTags {
		t.Errorf("validateCustomTags returned %d tags, want the cap of %d", len(got), maxCustomTags)
	}
}
