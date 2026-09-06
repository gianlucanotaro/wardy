package ai

import (
	"testing"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// These are recorded/hand-built fixtures standing in for real model output —
// per CLAUDE.md, AI response parsing is tested against fixtures, never live
// API calls.

const cleanJSON = `{
  "category": "top",
  "subcategory": "t-shirt",
  "color_primary": "blue",
  "color_secondary": "white",
  "pattern": "striped",
  "material": "cotton",
  "fit": "regular",
  "formality": 2,
  "warmth": 2,
  "seasons": ["summer", "spring"],
  "occasions": ["casual"]
}`

func TestParseTagResponse_Clean(t *testing.T) {
	got, err := parseTagResponse(cleanJSON)
	if err != nil {
		t.Fatalf("parseTagResponse: %v", err)
	}
	if got.Category != "top" || got.Subcategory != "t-shirt" || got.ColorPrimary != "blue" {
		t.Errorf("unexpected result: %+v", got)
	}
	if len(got.Seasons) != 2 || got.Seasons[0] != "summer" {
		t.Errorf("seasons not parsed: %+v", got.Seasons)
	}
}

func TestParseTagResponse_MarkdownFencedWithLang(t *testing.T) {
	got, err := parseTagResponse("```json\n" + cleanJSON + "\n```")
	if err != nil {
		t.Fatalf("parseTagResponse: %v", err)
	}
	if got.Category != "top" {
		t.Errorf("fenced response not parsed: %+v", got)
	}
}

func TestParseTagResponse_MarkdownFencedPlain(t *testing.T) {
	got, err := parseTagResponse("```\n" + cleanJSON + "\n```")
	if err != nil {
		t.Fatalf("parseTagResponse: %v", err)
	}
	if got.Category != "top" {
		t.Errorf("fenced response not parsed: %+v", got)
	}
}

func TestParseTagResponse_LeadingTrailingWhitespace(t *testing.T) {
	got, err := parseTagResponse("\n\n  " + cleanJSON + "  \n")
	if err != nil {
		t.Fatalf("parseTagResponse: %v", err)
	}
	if got.Category != "top" {
		t.Errorf("whitespace-padded response not parsed: %+v", got)
	}
}

func TestParseTagResponse_IgnoresUnknownFields(t *testing.T) {
	withExtra := `{"category":"top","subcategory":"t-shirt","color_primary":"blue",
		"color_secondary":"","pattern":"solid","material":"cotton","fit":"regular",
		"formality":2,"warmth":2,"seasons":["summer"],"occasions":["casual"],
		"confidence":0.97,"reasoning":"looks like a plain blue tee"}`

	got, err := parseTagResponse(withExtra)
	if err != nil {
		t.Fatalf("parseTagResponse with unexpected extra fields: %v", err)
	}
	if got.Category != "top" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestParseTagResponse_MalformedJSON(t *testing.T) {
	cases := []string{
		`{"category": "top", "subcategory":`,      // truncated
		`{"category": "top" "subcategory": "x"}`,  // missing comma
		`not json at all, just a sentence.`,       // prose instead of JSON
		``,                                        // empty response
		`{"category": "top", "formality": "two"}`, // wrong type for formality
	}
	for _, c := range cases {
		if _, err := parseTagResponse(c); err == nil {
			t.Errorf("parseTagResponse(%q) = nil error, want an error", c)
		}
	}
}

const cleanSuggestJSON = `[
  {"item_ids": [1, 2], "rationale": "A breezy top with tailored trousers for the weather."},
  {"item_ids": [3, 4, 5], "rationale": "A dress layered with a jacket for a cooler evening."}
]`

func TestParseSuggestResponse_Clean(t *testing.T) {
	got, err := parseSuggestResponse(cleanSuggestJSON)
	if err != nil {
		t.Fatalf("parseSuggestResponse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(got))
	}
	if len(got[0].ItemIDs) != 2 || got[0].ItemIDs[0] != 1 || got[0].ItemIDs[1] != 2 {
		t.Errorf("unexpected item ids: %+v", got[0].ItemIDs)
	}
	if got[0].Rationale == "" {
		t.Error("rationale should not be empty")
	}
}

func TestParseSuggestResponse_MarkdownFenced(t *testing.T) {
	got, err := parseSuggestResponse("```json\n" + cleanSuggestJSON + "\n```")
	if err != nil {
		t.Fatalf("parseSuggestResponse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(got))
	}
}

func TestParseSuggestResponse_MalformedJSON(t *testing.T) {
	cases := []string{
		`[{"item_ids": [1, 2], "rationale":`, // truncated
		`not json at all.`,                   // prose instead of JSON
		``,                                   // empty response
		`{"item_ids": [1, 2]}`,               // object instead of array
	}
	for _, c := range cases {
		if _, err := parseSuggestResponse(c); err == nil {
			t.Errorf("parseSuggestResponse(%q) = nil error, want an error", c)
		}
	}
}

// TestParseSuggestResponse_HallucinatedIDsParseFine documents that parsing
// alone never rejects an id — a model can return syntactically valid JSON
// referencing an item id that was never in the candidate set. Validating
// ids against the candidate set is internal/suggest's job, not the
// parser's — see suggest.SuggestAI.
func TestParseSuggestResponse_HallucinatedIDsParseFine(t *testing.T) {
	got, err := parseSuggestResponse(`[{"item_ids": [9999], "rationale": "invented item"}]`)
	if err != nil {
		t.Fatalf("parseSuggestResponse: %v", err)
	}
	if len(got) != 1 || got[0].ItemIDs[0] != 9999 {
		t.Errorf("unexpected result: %+v", got)
	}
}

// TestParseThenValidate_RejectsOutOfVocabValues exercises the exact
// pipeline the worker runs: parse the model's text, then validate against
// the closed vocabularies. A model can return syntactically valid JSON
// with semantically invalid (hallucinated/out-of-vocab) values — parsing
// alone must not be mistaken for validation.
func TestParseThenValidate_RejectsOutOfVocabValues(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "invented category",
			json: `{"category":"sock","subcategory":"ankle-sock","color_primary":"white",
				"color_secondary":"","pattern":"solid","material":"cotton","fit":"regular",
				"formality":1,"warmth":2,"seasons":["summer"],"occasions":["home"]}`,
		},
		{
			name: "invented color",
			json: `{"category":"top","subcategory":"t-shirt","color_primary":"sage mist",
				"color_secondary":"","pattern":"solid","material":"cotton","fit":"regular",
				"formality":1,"warmth":2,"seasons":["summer"],"occasions":["casual"]}`,
		},
		{
			name: "formality out of range",
			json: `{"category":"top","subcategory":"t-shirt","color_primary":"white",
				"color_secondary":"","pattern":"solid","material":"cotton","fit":"regular",
				"formality":11,"warmth":2,"seasons":["summer"],"occasions":["casual"]}`,
		},
		{
			name: "invented season",
			json: `{"category":"top","subcategory":"t-shirt","color_primary":"white",
				"color_secondary":"","pattern":"solid","material":"cotton","fit":"regular",
				"formality":1,"warmth":2,"seasons":["monsoon"],"occasions":["casual"]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := parseTagResponse(c.json)
			if err != nil {
				t.Fatalf("parseTagResponse: %v (parsing itself should succeed on valid JSON)", err)
			}

			tags := taxonomy.Tags{
				Category:       taxonomy.Category(result.Category),
				Subcategory:    result.Subcategory,
				ColorPrimary:   result.ColorPrimary,
				ColorSecondary: result.ColorSecondary,
				Pattern:        taxonomy.Pattern(result.Pattern),
				Material:       taxonomy.Material(result.Material),
				Fit:            taxonomy.Fit(result.Fit),
				Formality:      result.Formality,
				Warmth:         result.Warmth,
				Seasons:        result.Seasons,
				Occasions:      result.Occasions,
			}
			if err := tags.Validate(); err == nil {
				t.Errorf("Validate() = nil, want the out-of-vocab value to be rejected")
			}
		})
	}
}
