package suggest

import (
	"strings"
	"testing"
)

func TestCleanPrompt(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "only whitespace", raw: " \n\t ", want: ""},
		{name: "plain", raw: "something comfy for a train ride", want: "something comfy for a train ride"},
		{name: "trims and collapses", raw: "  smart   but\nnot stiff  ", want: "smart but not stiff"},
		{name: "control characters become spaces", raw: "green\x00jacket\x1b[2J please", want: "green jacket [2J please"},
		{name: "keeps accents and emoji", raw: "élégant 🎉", want: "élégant 🎉"},
		{name: "at the cap", raw: strings.Repeat("é", MaxPromptLength), want: strings.Repeat("é", MaxPromptLength)},
		{name: "cap counts after collapsing", raw: strings.Repeat("a", MaxPromptLength) + "\n\n\n", want: strings.Repeat("a", MaxPromptLength)},
		{name: "over the cap", raw: strings.Repeat("a", MaxPromptLength+1), wantErr: true},
		{name: "invalid utf-8", raw: "abc\xff", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CleanPrompt(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CleanPrompt(%q) = %q, want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanPrompt(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("CleanPrompt(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
