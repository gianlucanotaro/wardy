package ai

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTagResponse_Styles(t *testing.T) {
	got, err := parseTagResponse(`{"category":"top","subcategory":"t-shirt","color_primary":"blue",
		"pattern":"solid","material":"cotton","fit":"regular",
		"formality_min":1,"formality_max":3,"warmth_min":1,"warmth_max":5,
		"styles":["minimal","streetwear"]}`)
	if err != nil {
		t.Fatalf("parseTagResponse: %v", err)
	}
	if !reflect.DeepEqual(got.Styles, []string{"minimal", "streetwear"}) {
		t.Errorf("Styles = %v", got.Styles)
	}
}

func TestParseItemStylesResponse(t *testing.T) {
	got, err := parseItemStylesResponse("```json\n{\"styles\": [\"classic\", \"preppy\"]}\n```")
	if err != nil {
		t.Fatalf("parseItemStylesResponse: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"classic", "preppy"}) {
		t.Errorf("styles = %v", got)
	}
	if _, err := parseItemStylesResponse("the styles are classic"); err == nil {
		t.Error("prose parsed without error, want one")
	}
}

// Both prompts must offer the full style vocabulary, so a valid answer is
// always possible.
func TestPromptsListStyles(t *testing.T) {
	for name, prompt := range map[string]string{"tagging": taggingSystemPrompt, "styles": itemStylesSystemPrompt} {
		if !strings.Contains(prompt, "smart-casual") || !strings.Contains(prompt, "avant-garde") {
			t.Errorf("%s prompt doesn't list the styles", name)
		}
	}
}

func TestMockStylesAreValidAndStable(t *testing.T) {
	a := mockStyles([]byte("photo one"))
	if !reflect.DeepEqual(a, mockStyles([]byte("photo one"))) {
		t.Error("the same photo got different styles")
	}
	if len(a) < 1 || len(a) > 2 {
		t.Errorf("mockStyles = %v, want 1 or 2", a)
	}
}
