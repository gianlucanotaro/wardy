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
	"slices"
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

	reason, err := diagnoseNoMatch(ctx, s, userID, filter, targetWarmth)
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
func SuggestAI(ctx context.Context, s *store.Store, provider ai.Provider, aiEnabled bool, monthlyBudgetUSD float64, dailyCap int, userID int64, occasion taxonomy.Occasion, sc ai.SuggestContext, forecast *weather.Forecast, northernHemisphere bool, now time.Time) (Result, error) {
	candidates, filter, targetWarmth, err := gatherCandidates(ctx, s, userID, occasion, forecast, northernHemisphere, now)
	if err != nil {
		return Result{}, err
	}

	rulesOutfits := assembleOutfits(groupByCategory(candidates), targetWarmth)
	if len(rulesOutfits) == 0 {
		reason, err := diagnoseNoMatch(ctx, s, userID, filter, targetWarmth)
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

	suggestions, usage, err := provider.SuggestOutfits(ctx, toAICandidates(candidates), string(occasion), sc)
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

	outfits := validateAISuggestions(suggestions, candidates, targetWarmth)
	if len(outfits) == 0 {
		slog.Warn("suggest: ai returned no outfits with valid candidate ids, falling back to rules")
		return rulesResult, nil
	}
	if len(outfits) > MaxOutfits {
		outfits = outfits[:MaxOutfits]
	}

	return Result{Outfits: outfits, Source: SourceAI}, nil
}

// gatherCandidates runs the shared pre-filter and sampling that both
// Suggest and SuggestAI build their outfits from, so an AI suggestion is
// always drawn from the exact same pool as its rules-based fallback.
func gatherCandidates(ctx context.Context, s *store.Store, userID int64, occasion taxonomy.Occasion, forecast *weather.Forecast, northernHemisphere bool, now time.Time) ([]store.Item, store.CandidateFilter, *int, error) {
	formality := OccasionFormality(occasion)

	var targetWarmth *int
	if forecast != nil {
		w := TargetWarmth(*forecast)
		targetWarmth = &w
	}

	filter := store.CandidateFilter{
		FormalityMin:         formality.Min,
		FormalityMax:         formality.Max,
		ExcludeWornSinceDays: ExcludeWornSinceDays,
	}

	candidates, err := candidateItems(ctx, s, userID, filter, targetWarmth)
	if err != nil {
		return nil, store.CandidateFilter{}, nil, fmt.Errorf("query candidates: %w", err)
	}

	preferTagged(candidates, occasion, CurrentSeason(now, northernHemisphere))

	if len(candidates) > MaxCandidates {
		candidates = sampleAcrossCategories(candidates, MaxCandidates)
	}

	return candidates, filter, targetWarmth, nil
}

// candidateItems is the store query plus the per-category weather check
// (see FitsWeather), which needs Go rather than SQL. targetWarmth nil
// means weather is unavailable: suggest anyway, unfiltered.
func candidateItems(ctx context.Context, s *store.Store, userID int64, filter store.CandidateFilter, targetWarmth *int) ([]store.Item, error) {
	items, err := s.CandidateItems(ctx, userID, filter)
	if err != nil || targetWarmth == nil {
		return items, err
	}
	kept := items[:0]
	for _, it := range items {
		if FitsWeather(it, *targetWarmth) {
			kept = append(kept, it)
		}
	}
	return kept, nil
}

// preferTagged reorders items so the ones the user tagged for today's
// occasion come first, then the ones tagged for the current season. Tags
// are preferences, not filters: untagged pieces still qualify, they just
// get picked after the tagged ones. Stable, so ties keep newest-first.
func preferTagged(items []store.Item, occasion taxonomy.Occasion, season taxonomy.Season) {
	score := func(it store.Item) int {
		n := 0
		if slices.Contains(it.Occasions, string(occasion)) {
			n += 2
		}
		if slices.Contains(it.Seasons, string(season)) {
			n++
		}
		return n
	}
	sort.SliceStable(items, func(i, j int) bool { return score(items[i]) > score(items[j]) })
}

// toAICandidates strips items down to ids and the relevant attributes for
// the compact JSON sent to the model — no paths, no timestamps
// (docs/ai.md).
func toAICandidates(items []store.Item) []ai.SuggestionCandidate {
	out := make([]ai.SuggestionCandidate, len(items))
	for i, it := range items {
		out[i] = ai.SuggestionCandidate{
			ID:           it.ID,
			Name:         it.Name,
			Category:     it.Category,
			Subcategory:  it.Subcategory,
			ColorPrimary: it.ColorPrimary,
			FormalityMin: it.FormalityMin,
			FormalityMax: it.FormalityMax,
			WarmthMin:    it.WarmthMin,
			WarmthMax:    it.WarmthMax,
			CustomTags:   it.CustomTags,
		}
	}
	return out
}

// validateAISuggestions drops every outfit that references an item id
// outside candidates — the model can hallucinate ids, and this is the
// only thing standing between that and a broken card (docs/ai.md:
// "validate every returned id ... drop invalid outfits rather than
// rendering a broken card").
//
// It also drops outfits that break the rules pass's outfit-level rules
// (outfitWorks): the model only sees individually-fitting pieces, so it
// can still pair a shirt with no layer over it on a cold day.
func validateAISuggestions(suggestions []ai.AISuggestion, candidates []store.Item, targetWarmth *int) []Outfit {
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
		if !valid || len(items) == 0 || !outfitWorks(items, targetWarmth) {
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
// or a dress as the base, then layers for the weather, then shoes. Every
// added piece has to keep the outfit's formality coherent (see
// coherent). No AI involved — just picking distinct items per slot, in
// candidate order, so the suggestions vary.
func assembleOutfits(byCategory map[string][]store.Item, targetWarmth *int) []Outfit {
	tops := byCategory[string(taxonomy.CategoryTop)]
	bottoms := byCategory[string(taxonomy.CategoryBottom)]
	dresses := byCategory[string(taxonomy.CategoryDress)]
	outerwear := byCategory[string(taxonomy.CategoryOuterwear)]
	shoes := byCategory[string(taxonomy.CategoryShoes)]

	var bases [][]store.Item
	usedBottoms := make(map[int64]bool)
	for _, top := range tops {
		// Prefer a bottom no earlier base used, so suggestions vary; fall
		// back to reusing one rather than dropping the top.
		bottom := pick(bottoms, []store.Item{top}, 0, func(b store.Item) bool { return !usedBottoms[b.ID] })
		if bottom == nil {
			bottom = pick(bottoms, []store.Item{top}, 0, nil)
		}
		if bottom == nil {
			continue
		}
		usedBottoms[bottom.ID] = true
		bases = append(bases, []store.Item{top, *bottom})
	}
	for _, d := range dresses {
		bases = append(bases, []store.Item{d})
	}

	outfits := make([]Outfit, 0, MaxOutfits)
	for i, base := range bases {
		if len(outfits) >= MaxOutfits {
			break
		}

		items, layered, ok := layerForWeather(base, tops, outerwear, i, targetWarmth)
		if !ok {
			continue
		}
		if shoe := pick(shoes, items, i, nil); shoe != nil {
			items = append(items, *shoe)
		}

		outfits = append(outfits, Outfit{Items: items, Note: outfitNote(layered)})
	}

	return outfits
}

// layerForWeather adds a mid layer and/or outerwear to base until its
// warmest upper-body piece reaches the needed warmth: a shirt in winter
// gets a knit and a coat over it. ok is false when the wardrobe can't make
// this base warm enough, so the caller skips it. With no weather data the
// base is returned as is.
func layerForWeather(base, tops, outerwear []store.Item, offset int, targetWarmth *int) (items []store.Item, layered, ok bool) {
	items = append([]store.Item{}, base...)
	if targetWarmth == nil {
		return items, false, true
	}
	need := neededWarmth(*targetWarmth)

	hasTop := slices.ContainsFunc(items, func(it store.Item) bool { return it.Category == string(taxonomy.CategoryTop) })
	if hasTop && upperWarmth(items) < need {
		mid := pick(tops, items, offset, func(t store.Item) bool { return pieceWarmth(t) >= need })
		if mid != nil {
			items = append(items, *mid)
			layered = true
		}
	}

	if *targetWarmth >= OuterwearFromWarmth || upperWarmth(items) < need {
		if coat := pick(outerwear, items, offset, nil); coat != nil {
			items = append(items, *coat)
			layered = true
		}
	}

	return items, layered, upperWarmth(items) >= need
}

// upperWarmth is the warmth of the warmest top, dress or outerwear in
// items — layers worn together feel at least as warm as the warmest one.
func upperWarmth(items []store.Item) int {
	w := 0
	for _, it := range items {
		switch taxonomy.Category(it.Category) {
		case taxonomy.CategoryTop, taxonomy.CategoryDress, taxonomy.CategoryOuterwear:
			w = max(w, pieceWarmth(it))
		}
	}
	return w
}

// pick returns the first item of list, starting at offset and wrapping
// around, that isn't already in outfit, keeps outfit coherent, and passes
// ok (nil accepts any). nil if none qualifies.
func pick(list, outfit []store.Item, offset int, ok func(store.Item) bool) *store.Item {
	for n := range list {
		cand := list[(offset+n)%len(list)]
		if slices.ContainsFunc(outfit, func(it store.Item) bool { return it.ID == cand.ID }) {
			continue
		}
		if ok != nil && !ok(cand) {
			continue
		}
		if !coherent(append(append([]store.Item{}, outfit...), cand)) {
			continue
		}
		return &cand
	}
	return nil
}

// coherent reports whether items' formality ranges come within
// MaxFormalitySpread of each other: dressing one piece up works, mixing
// black tie with loungewear doesn't. The gap is between the most formal
// piece's lowest setting and the least formal piece's highest one.
func coherent(items []store.Item) bool {
	if len(items) == 0 {
		return true
	}
	highestMin, lowestMax := items[0].FormalityMin, items[0].FormalityMax
	for _, it := range items[1:] {
		highestMin, lowestMax = max(highestMin, it.FormalityMin), min(lowestMax, it.FormalityMax)
	}
	return highestMin-lowestMax <= MaxFormalitySpread
}

// outfitWorks applies the same outfit-level rules the rules pass builds
// by to an outfit someone else (the AI) assembled: formality coherent,
// and warm enough up top once its layers are counted.
func outfitWorks(items []store.Item, targetWarmth *int) bool {
	if !coherent(items) {
		return false
	}
	return targetWarmth == nil || upperWarmth(items) >= neededWarmth(*targetWarmth)
}

func outfitNote(layered bool) string {
	if layered {
		return "Matches today's weather and occasion — layered for the cold."
	}
	return "Matches today's weather and occasion."
}

// diagnoseNoMatch relaxes one constraint at a time, in the order a user
// would most plausibly want explained, and reports the first one that
// would have produced an outfit.
func diagnoseNoMatch(ctx context.Context, s *store.Store, userID int64, base store.CandidateFilter, targetWarmth *int) (string, error) {
	works := func(f store.CandidateFilter, warmth *int) (bool, error) {
		items, err := candidateItems(ctx, s, userID, f, warmth)
		if err != nil {
			return false, err
		}
		return len(assembleOutfits(groupByCategory(items), warmth)) > 0, nil
	}

	relaxations := []struct {
		apply  func(*store.CandidateFilter, **int)
		reason string
	}{
		{
			func(f *store.CandidateFilter, _ **int) { f.ExcludeWornSinceDays = 0 },
			"You've worn everything else that matches recently — try again in a few days, or add more to your wardrobe.",
		},
		{
			func(_ *store.CandidateFilter, w **int) { *w = nil },
			"Nothing in your wardrobe layers up right for today's weather — add a few pieces for this temperature.",
		},
		{
			func(f *store.CandidateFilter, _ **int) {
				f.FormalityMin, f.FormalityMax = taxonomy.MinScale, taxonomy.MaxScale
			},
			"Not enough pieces at the right formality for this occasion — add a few, or check their formality tags.",
		},
	}

	for _, r := range relaxations {
		candidate, warmth := base, targetWarmth
		r.apply(&candidate, &warmth)

		ok, err := works(candidate, warmth)
		if err != nil {
			return "", err
		}
		if ok {
			return r.reason, nil
		}
	}

	return "Not enough tagged items yet to suggest an outfit — tag a few more tops, bottoms, or dresses.", nil
}
