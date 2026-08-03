// Package suggest is the rules-based outfit pre-filter: deterministic,
// zero-AI logic that is both the foundation phase 8's LLM suggestions
// build on and the permanent fallback whenever AI is unavailable. See
// docs/ai.md and CLAUDE.md's build order for why this comes before phase
// 8, not after.
package suggest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

const (
	// MaxCandidates caps the pool before assembly (docs/ai.md's "cap at
	// ~60 candidates, sampled across categories") — even with no AI call
	// to protect a token budget for, an enormous pool is still wasteful
	// to assemble from.
	MaxCandidates = 60

	// ExcludeWornSinceDays: don't resuggest something worn this recently.
	ExcludeWornSinceDays = 7

	// MaxOutfits is how many complete outfits to assemble and return.
	MaxOutfits = 3
)

// Outfit is one assembled combination of items, plus a short factual note
// — not an AI-written rationale, that's phase 8's job.
type Outfit struct {
	Items []store.Item
	Note  string
}

// Result is what Suggest returns: either outfits, or (if nothing usable
// was assembled) a plain-language reason naming the tightest constraint,
// per docs/frontend.md: "say which constraint is too tight rather than
// showing an empty screen."
type Result struct {
	Outfits       []Outfit
	NoMatchReason string
}

// Suggest runs the pre-filter for occasion. forecast is nil if weather is
// unavailable (no location set, or the fetch failed) — Suggest still
// returns outfits in that case, just without warmth-matching.
// northernHemisphere should be the sign of the user's home latitude. now
// is passed in explicitly (rather than calling time.Now() internally) so
// the season calculation is deterministic in tests regardless of what
// day it actually is when they run.
func Suggest(ctx context.Context, s *store.Store, userID int64, occasion taxonomy.Occasion, forecast *weather.Forecast, northernHemisphere bool, now time.Time) (Result, error) {
	season := CurrentSeason(now, northernHemisphere)
	targetFormality := TargetFormality(occasion)

	var targetWarmth *int
	if forecast != nil {
		w := TargetWarmth(*forecast)
		targetWarmth = &w
	}

	occasionStr := string(occasion)
	seasonStr := string(season)
	filter := store.CandidateFilter{
		Occasion:             &occasionStr,
		Season:               &seasonStr,
		TargetFormality:      targetFormality,
		FormalityWindow:      MatchWindow,
		TargetWarmth:         targetWarmth,
		WarmthWindow:         MatchWindow,
		ExcludeWornSinceDays: ExcludeWornSinceDays,
	}

	candidates, err := s.CandidateItems(ctx, userID, filter)
	if err != nil {
		return Result{}, fmt.Errorf("query candidates: %w", err)
	}

	if len(candidates) > MaxCandidates {
		candidates = sampleAcrossCategories(candidates, MaxCandidates)
	}

	outfits := assembleOutfits(groupByCategory(candidates), targetWarmth)
	if len(outfits) > 0 {
		return Result{Outfits: outfits}, nil
	}

	reason, err := diagnoseNoMatch(ctx, s, userID, filter)
	if err != nil {
		return Result{}, fmt.Errorf("diagnose no match: %w", err)
	}
	return Result{NoMatchReason: reason}, nil
}

func groupByCategory(items []store.Item) map[string][]store.Item {
	m := make(map[string][]store.Item)
	for _, it := range items {
		m[it.Category] = append(m[it.Category], it)
	}
	return m
}

// sampleAcrossCategories round-robins across categories rather than just
// truncating, so a category with many tagged items can't crowd out one
// with few (docs/ai.md: "sample across categories so every slot has
// options").
func sampleAcrossCategories(items []store.Item, max int) []store.Item {
	byCategory := groupByCategory(items)

	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories) // deterministic order

	result := make([]store.Item, 0, max)
	next := make(map[string]int)
	for len(result) < max {
		addedAny := false
		for _, cat := range categories {
			i := next[cat]
			if i >= len(byCategory[cat]) {
				continue
			}
			result = append(result, byCategory[cat][i])
			next[cat] = i + 1
			addedAny = true
			if len(result) >= max {
				break
			}
		}
		if !addedAny {
			break
		}
	}
	return result
}

// assembleOutfits builds up to MaxOutfits combinations: a top+bottom pair
// or a dress as the base, plus shoes and (if it's cold) outerwear when
// available. No AI involved — just picking distinct items per slot so
// the suggestions vary.
func assembleOutfits(byCategory map[string][]store.Item, targetWarmth *int) []Outfit {
	tops := byCategory[string(taxonomy.CategoryTop)]
	bottoms := byCategory[string(taxonomy.CategoryBottom)]
	dresses := byCategory[string(taxonomy.CategoryDress)]
	outerwear := byCategory[string(taxonomy.CategoryOuterwear)]
	shoes := byCategory[string(taxonomy.CategoryShoes)]

	needsOuterwear := targetWarmth != nil && *targetWarmth >= 4

	var bases [][]store.Item
	for i := 0; i < len(tops) && i < len(bottoms); i++ {
		bases = append(bases, []store.Item{tops[i], bottoms[i]})
	}
	for _, d := range dresses {
		bases = append(bases, []store.Item{d})
	}

	outfits := make([]Outfit, 0, MaxOutfits)
	for i, base := range bases {
		if len(outfits) >= MaxOutfits {
			break
		}

		items := append([]store.Item{}, base...)
		if len(shoes) > 0 {
			items = append(items, shoes[i%len(shoes)])
		}
		if needsOuterwear && len(outerwear) > 0 {
			items = append(items, outerwear[i%len(outerwear)])
		}

		outfits = append(outfits, Outfit{Items: items, Note: outfitNote(needsOuterwear)})
	}

	return outfits
}

func outfitNote(includesOuterwear bool) string {
	if includesOuterwear {
		return "Matches today's weather and occasion — layered for the cold."
	}
	return "Matches today's weather and occasion."
}

// diagnoseNoMatch relaxes one constraint at a time, in the order a user
// would most plausibly want explained, and reports the first one that
// would have produced a usable outfit base (top+bottom or dress).
func diagnoseNoMatch(ctx context.Context, s *store.Store, userID int64, base store.CandidateFilter) (string, error) {
	hasBase := func(f store.CandidateFilter) (bool, error) {
		items, err := s.CandidateItems(ctx, userID, f)
		if err != nil {
			return false, err
		}
		byCategory := groupByCategory(items)
		ok := (len(byCategory[string(taxonomy.CategoryTop)]) > 0 && len(byCategory[string(taxonomy.CategoryBottom)]) > 0) ||
			len(byCategory[string(taxonomy.CategoryDress)]) > 0
		return ok, nil
	}

	relaxations := []struct {
		apply  func(*store.CandidateFilter)
		reason string
	}{
		{
			func(f *store.CandidateFilter) { f.ExcludeWornSinceDays = 0 },
			"You've worn everything else that matches recently — try again in a few days, or add more to your wardrobe.",
		},
		{
			func(f *store.CandidateFilter) { f.TargetWarmth = nil },
			"Nothing matches today's weather yet — consider tagging items across a wider warmth range.",
		},
		{
			func(f *store.CandidateFilter) { f.FormalityWindow = taxonomy.MaxScale },
			"Nothing matches that occasion's formality yet — try tagging more items for it.",
		},
		{
			func(f *store.CandidateFilter) { f.Season = nil },
			"Nothing tagged for the current season yet.",
		},
		{
			func(f *store.CandidateFilter) { f.Occasion = nil },
			"Nothing tagged for this occasion yet.",
		},
	}

	for _, r := range relaxations {
		candidate := base
		r.apply(&candidate)

		ok, err := hasBase(candidate)
		if err != nil {
			return "", err
		}
		if ok {
			return r.reason, nil
		}
	}

	return "Not enough tagged items yet to suggest an outfit — tag a few more tops, bottoms, or dresses.", nil
}
