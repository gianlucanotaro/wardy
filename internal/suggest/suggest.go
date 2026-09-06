// Package suggest is the outfit suggestion domain: a deterministic,
// zero-AI rules pre-filter (Suggest) that is both the foundation phase 8's
// LLM suggestions (SuggestAI) build on and the permanent fallback whenever
// AI is unavailable, over budget, or over its per-user daily cap. See
// docs/ai.md and CLAUDE.md's build order for why the rules pass exists
// before the AI layer, not after.
package suggest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
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

	// DailySuggestionCap is the per-user daily limit on AI suggestion
	// calls (docs/ai.md: "~20 suggestions"), independent of MaxCandidates
	// and MaxOutfits, which bound the scope of a single call.
	DailySuggestionCap = 20
)

// Result.Source values. SourceRulesFallback covers both the deterministic
// Suggest() path and any SuggestAI() call that degraded to it; the empty
// value (no outfits, NoMatchReason set) is the third UI state per
// docs/frontend.md.
const (
	SourceAI            = "ai"
	SourceRulesFallback = "rules_fallback"
)

// Outfit is one assembled combination of items, plus a short note — the
// deterministic factual note from the rules pass, or the AI's rationale
// when Result.Source is SourceAI.
type Outfit struct {
	Items []store.Item
	Note  string
}

// Result is what Suggest and SuggestAI return: either outfits (tagged
// with Source so the UI can distinguish an AI-written rationale from the
// rules-only fallback), or — if nothing usable was assembled — a
// plain-language reason naming the tightest constraint, per
// docs/frontend.md: "say which constraint is too tight rather than
// showing an empty screen."
type Result struct {
	Outfits       []Outfit
	Source        string
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
	candidates, filter, targetWarmth, err := gatherCandidates(ctx, s, userID, occasion, forecast, northernHemisphere, now)
	if err != nil {
		return Result{}, err
	}

	outfits := assembleOutfits(groupByCategory(candidates), targetWarmth)
	if len(outfits) > 0 {
		return Result{Outfits: outfits, Source: SourceRulesFallback}, nil
	}

	reason, err := diagnoseNoMatch(ctx, s, userID, filter)
	if err != nil {
		return Result{}, fmt.Errorf("diagnose no match: %w", err)
	}
	return Result{NoMatchReason: reason}, nil
}

// SuggestAI layers claude-sonnet-5-written rationale on top of the same
// candidate pool the rules pass uses. It always computes the rules result
// first — that becomes both the permanent fallback and, when nothing
// matches at all, the answer outright, since a rules-based dead end means
// there was nothing for the AI to work with either. It falls back to the
// rules result (never an error) when aiEnabled is false, the monthly
// budget or daily cap is exceeded, the API call fails, or every returned
// outfit turns out to reference an id outside the candidate set — models
// hallucinate ids (docs/ai.md).
func SuggestAI(ctx context.Context, s *store.Store, provider ai.Provider, aiEnabled bool, monthlyBudgetUSD float64, dailyCap int, userID int64, occasion taxonomy.Occasion, mood, timeOfDay string, forecast *weather.Forecast, northernHemisphere bool, now time.Time) (Result, error) {
	candidates, filter, targetWarmth, err := gatherCandidates(ctx, s, userID, occasion, forecast, northernHemisphere, now)
	if err != nil {
		return Result{}, err
	}

	rulesOutfits := assembleOutfits(groupByCategory(candidates), targetWarmth)
	if len(rulesOutfits) == 0 {
		reason, err := diagnoseNoMatch(ctx, s, userID, filter)
		if err != nil {
			return Result{}, fmt.Errorf("diagnose no match: %w", err)
		}
		return Result{NoMatchReason: reason}, nil
	}
	rulesResult := Result{Outfits: rulesOutfits, Source: SourceRulesFallback}

	if !aiEnabled {
		return rulesResult, nil
	}

	if monthlyBudgetUSD > 0 {
		spend, err := s.MonthlySpend(ctx)
		if err != nil {
			slog.Error("suggest: check monthly budget", "error", err)
			return rulesResult, nil
		}
		if spend >= monthlyBudgetUSD {
			slog.Warn("suggest: over monthly AI budget, falling back to rules", "spend", spend, "budget", monthlyBudgetUSD)
			return rulesResult, nil
		}
	}

	if dailyCap > 0 {
		count, err := s.DailyUsageCount(ctx, userID, ai.SuggestionModelName)
		if err != nil {
			slog.Error("suggest: check daily suggestion cap", "error", err)
			return rulesResult, nil
		}
		if count >= dailyCap {
			slog.Warn("suggest: over daily suggestion cap, falling back to rules", "user_id", userID, "count", count, "cap", dailyCap)
			return rulesResult, nil
		}
	}

	suggestions, usage, err := provider.SuggestOutfits(ctx, toAICandidates(candidates), string(occasion), mood, timeOfDay)
	if usage.Model != "" {
		if logErr := s.LogUsage(ctx, userID, usage.Model, usage.InputTokens, usage.OutputTokens, usage.EstCostUSD); logErr != nil {
			slog.Error("suggest: log usage", "error", logErr)
		}
	}
	if err != nil {
		if !errors.Is(err, ai.ErrDisabled) {
			slog.Warn("suggest: ai suggest failed, falling back to rules", "error", err)
		}
		return rulesResult, nil
	}

	outfits := validateAISuggestions(suggestions, candidates)
	if len(outfits) == 0 {
		slog.Warn("suggest: ai returned no outfits with valid candidate ids, falling back to rules")
		return rulesResult, nil
	}
	if len(outfits) > MaxOutfits {
		outfits = outfits[:MaxOutfits]
	}

	return Result{Outfits: outfits, Source: SourceAI}, nil
}

// gatherCandidates runs the shared SQL pre-filter and sampling that both
// Suggest and SuggestAI build their outfits from, so an AI suggestion is
// always drawn from the exact same pool as its rules-based fallback.
func gatherCandidates(ctx context.Context, s *store.Store, userID int64, occasion taxonomy.Occasion, forecast *weather.Forecast, northernHemisphere bool, now time.Time) ([]store.Item, store.CandidateFilter, *int, error) {
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
		return nil, store.CandidateFilter{}, nil, fmt.Errorf("query candidates: %w", err)
	}

	if len(candidates) > MaxCandidates {
		candidates = sampleAcrossCategories(candidates, MaxCandidates)
	}

	return candidates, filter, targetWarmth, nil
}

// toAICandidates strips items down to ids and the relevant attributes for
// the compact JSON sent to the model — no paths, no timestamps
// (docs/ai.md).
func toAICandidates(items []store.Item) []ai.SuggestionCandidate {
	out := make([]ai.SuggestionCandidate, len(items))
	for i, it := range items {
		out[i] = ai.SuggestionCandidate{
			ID:           it.ID,
			Category:     it.Category,
			Subcategory:  it.Subcategory,
			ColorPrimary: it.ColorPrimary,
			Formality:    it.Formality,
			Warmth:       it.Warmth,
		}
	}
	return out
}

// validateAISuggestions drops every outfit that references an item id
// outside candidates — the model can hallucinate ids, and this is the
// only thing standing between that and a broken card (docs/ai.md:
// "validate every returned id ... drop invalid outfits rather than
// rendering a broken card").
func validateAISuggestions(suggestions []ai.AISuggestion, candidates []store.Item) []Outfit {
	byID := make(map[int64]store.Item, len(candidates))
	for _, it := range candidates {
		byID[it.ID] = it
	}

	outfits := make([]Outfit, 0, len(suggestions))
	for _, sug := range suggestions {
		items := make([]store.Item, 0, len(sug.ItemIDs))
		valid := true
		for _, id := range sug.ItemIDs {
			it, ok := byID[id]
			if !ok {
				valid = false
				break
			}
			items = append(items, it)
		}
		if !valid || len(items) == 0 {
			continue
		}
		outfits = append(outfits, Outfit{Items: items, Note: sug.Rationale})
	}
	return outfits
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
