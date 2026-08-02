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
