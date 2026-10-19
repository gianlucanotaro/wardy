// Package inspo turns a user's inspiration board (#49) into a style
// profile: plain counting over the fits' tags, so it works with
// AI_PROVIDER=off. The profile goes into the suggestion prompt as a
// preference (ai.SuggestContext.Style) — never a filter on the
// candidates, which stay the rules pre-filter's.
package inspo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// MinFitsForSuggestions is how many tagged fits a board needs before its
// profile is used for suggestions — fewer than this says more about one
// photo than about a style.
const MinFitsForSuggestions = 5

// How many of each list the profile keeps: the dominant ones, which is
// what a style is.
const (
	topStyles = 3
	topColors = 6
	topPieces = 8
	topFits   = 2
)

// Count is one value and how many fits carry it.
type Count struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Profile is what a board adds up to. Tagged counts the fits with at least
// one style, colour or piece; every Count is out of Tagged.
type Profile struct {
	Fits      int     `json:"fits"`
	Tagged    int     `json:"tagged"`
	Styles    []Count `json:"styles"`
	Colors    []Count `json:"colors"`
	Pieces    []Count `json:"pieces"`
	FitShapes []Count `json:"fit_shapes"`
	// Formality is the average middle of the fits' formality ranges, 0
	// when none has one.
	Formality float64 `json:"formality"`
}

// Build adds up fits into a Profile.
func Build(fits []store.InspoFit) Profile {
	p := Profile{Fits: len(fits)}
	styles, colors, pieces, shapes := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	formalitySum, formalityN := 0.0, 0
	for _, f := range fits {
		if len(f.Styles) == 0 && len(f.Colors) == 0 && len(f.Pieces) == 0 {
			continue
		}
		p.Tagged++
		for _, v := range f.Styles {
			styles[v]++
		}
		for _, v := range f.Colors {
			colors[v]++
		}
		for _, v := range f.Pieces {
			pieces[v]++
		}
		if f.Fit != "" && f.Fit != "not-applicable" {
			shapes[f.Fit]++
		}
		if f.FormalityMin > 0 {
			formalitySum += float64(f.FormalityMin+f.FormalityMax) / 2
			formalityN++
		}
	}
	p.Styles = top(styles, topStyles)
	p.Colors = top(colors, topColors)
	p.Pieces = top(pieces, topPieces)
	p.FitShapes = top(shapes, topFits)
	if formalityN > 0 {
		p.Formality = formalitySum / float64(formalityN)
	}
	return p
}

// top returns the n most common values, most common first, ties
// alphabetical so the profile (and its Key) is deterministic.
func top(counts map[string]int, n int) []Count {
	out := make([]Count, 0, len(counts))
	for v, c := range counts {
		out = append(out, Count{Value: v, Count: c})
	}
	slices.SortFunc(out, func(a, b Count) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Value, b.Value)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// UsableForSuggestions reports whether the profile has enough tagged fits
// to say something about the user's style.
func (p Profile) UsableForSuggestions() bool {
	return p.Tagged >= MinFitsForSuggestions
}

// Key identifies the profile's content, so a cached style summary is only
// rewritten when the profile actually changed — not on every edit that
// leaves the top styles, colours and pieces where they were.
func (p Profile) Key() string {
	b, _ := json.Marshal(struct {
		Styles, Colors, Pieces, FitShapes []Count
		Formality                         string
	}{p.Styles, p.Colors, p.Pieces, p.FitShapes, fmt.Sprintf("%.1f", p.Formality)})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// Describe is the profile as compact text: what the style summary is
// written from, and (with the summary) what the suggestion model sees.
// Shares are percentages of tagged fits.
func (p Profile) Describe() string {
	if p.Tagged == 0 {
		return ""
	}
	pct := func(cs []Count) string {
		parts := make([]string, len(cs))
		for i, c := range cs {
			parts[i] = fmt.Sprintf("%s %d%%", c.Value, c.Count*100/p.Tagged)
		}
		return strings.Join(parts, ", ")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From %d outfit photos the user likes.", p.Tagged)
	if len(p.Styles) > 0 {
		fmt.Fprintf(&b, " Styles: %s.", pct(p.Styles))
	}
	if len(p.Colors) > 0 {
		fmt.Fprintf(&b, " Colours: %s.", pct(p.Colors))
	}
	if len(p.Pieces) > 0 {
		fmt.Fprintf(&b, " Pieces: %s.", pct(p.Pieces))
	}
	if len(p.FitShapes) > 0 {
		fmt.Fprintf(&b, " Fit: %s.", pct(p.FitShapes))
	}
	if p.Formality > 0 {
		fmt.Fprintf(&b, " Formality around %.1f of 5.", p.Formality)
	}
	return b.String()
}

// Matches counts the fits that show a piece like item: the same kind of
// piece (subcategory) in the same colour. It's what the "LIKE 6 OF YOUR
// INSPO FITS" tag on a suggested piece says.
func Matches(item store.Item, fits []store.InspoFit) int {
	n := 0
	for _, f := range fits {
		if slices.Contains(f.Pieces, item.Subcategory) && slices.Contains(f.Colors, item.ColorPrimary) {
			n++
		}
	}
	return n
}
