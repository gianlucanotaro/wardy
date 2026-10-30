# AI integration

Read before touching tagging, suggestions, or anything calling the API.

`internal/ai` is an interface with three implementations selected by
`AI_PROVIDER`: `anthropic`, `mock`, `off`. It returns plain structs and knows
nothing about HTTP.

## Tagging — `claude-haiku-4-5-20251001`

Cheapest model; classification is its sweet spot. ~$0.003/item, so a 200-item
wardrobe costs well under a dollar, once.

- Background job. Never inline in the upload request.
- Send the **thumbnail** (~800px), not the original.
- JSON only. Strip markdown fences defensively.
- Validate every field against `docs/taxonomy.md`. Retry once, then fail the
  job and let the user tag manually.
- Set `ai_tagged_at`. When a user edits, set `user_edited_at` and never
  overwrite their values on a re-run.

## Suggestions — `claude-sonnet-5`

1. **Pre-filter** (SQL, plus a weather check in Go): the item's `formality`
   range overlaps the occasion's, not archived, not worn in last N days, and
   the forecast falls inside its `warmth` range. Occasion and season tags only rank candidates — tagged ones
   first — they never exclude. See `docs/taxonomy.md`.
2. Cap at ~60 candidates, sampled across categories so every slot has options.
   Items the user's request names (colour, category, subcategory, their own
   name — "a red top") are ranked first so the cap can't drop them; ranking
   only, like the tags.
3. Send compact JSON — ids and relevant attributes only. No paths, no
   timestamps.
4. Ask for 2–3 outfits: arrays of item ids + one-sentence rationale each.
5. **Validate every returned id** exists, belongs to this user, and was in the
   candidate set. Models hallucinate ids. Drop invalid outfits rather than
   rendering a broken card. Also drop outfits that break the outfit-level rules
   (formality spread, warm enough once layered) — the model never sees the
   forecast.

Mood and time of day are prompt context, not filters — they shape the rationale
and ranking, not the candidate set. Mood is a closed list (`suggest.Moods`:
sharp, easy, bold, quiet, cosy, playful); the server rejects anything else,
since it goes straight into the prompt. Outfits may carry one or two
accessories (never bags); Build puts them in its accessories slot.

The user can add a request in their own words (`?prompt=`, #41), e.g.
"dinner with my parents, smart but not stiff". Same context-not-filter rule:
it steers which candidates the model picks and the rationale answers it, but
the candidate set is still the rules pre-filter's. `suggest.CleanPrompt`
turns control characters into spaces and rejects anything over 280
characters; the prompt goes in JSON-quoted under "User's request", and the
system prompt tells the model it's a preference, never instructions. Never
log it. Item names are sent with the candidates so "the green jacket" can be
found.

## Inspiration board (#49)

Photos of other people's outfits, tagged, adding up to a **style profile**.
Nothing is trained or fine-tuned — the profile is counting:

1. **Tag each fit** — `tag_inspo` job, Haiku, thumbnail, same retry and
   validation rules as item tagging. Closed vocabularies only
   (`taxonomy.InspoTags`: styles, palette colours, pieces = item
   subcategories, fit, formality, seasons, occasions); too many values are
   trimmed, out-of-vocabulary ones fail the attempt. The prompt is about the
   clothes only: never describe, identify or rate the person.
2. **Profile** — `inspo.Build`: top styles, colours, pieces and fit as shares
   of tagged fits, average formality. Pure Go, works with `AI_PROVIDER=off`.
3. **Summary** — `summarize_style` job, Haiku, one sentence written from the
   counts (`Profile.Describe`), never the photos. Debounced 20s
   (`EnqueueJobOnce`) and only rewritten when `Profile.Key` changed; a
   summary for an older profile is never shown or sent.
4. **Suggestions** — with `users.use_inspo` on and at least
   `inspo.MinFitsForSuggestions` tagged fits, the summary + counts go in as
   JSON-quoted "User's style". Context, not filter, like mood and the
   request; the request wins over the style. `inspo.Matches` (same
   subcategory and colour) drives the "LIKE N OF YOUR INSPO FITS" tag.

Inspo tagging shares items' daily tagging cap; the summary is bounded by the
debounce and the key check.

**If the API is unreachable, fall back to rules-only.** The app must stay fully
usable with `AI_PROVIDER=off`.

## Weather — Open-Meteo

Free, no API key, no signup. Don't add a provider requiring registration.
Cache in-memory by rounded lat/lon, 1 hour TTL. The app sends the device's
current position (`?lat=&lon=` on `/api/weather` and `/api/suggest`, rounded to
~1 km) when the browser shares it; otherwise, or when the user picks "home" in
settings (per device, localStorage), the server uses the saved home location.
Location access is asked for, never required: a denial falls back to home, and
no location at all still suggests. On failure, suggest anyway with a note —
never block.

## Billing

**A Claude Pro/Max subscription does not include API access.** Separate
contracts, no shared balance. The subscription covers Claude Code for building
this; the app's runtime calls need a Console API key with prepaid credits.
Never route the app through subscription credentials.

`ANTHROPIC_API_KEY` from environment only. Never in the DB, never in a config
file, never sent to the frontend.

## Budget guards

Console credits with **auto-reload off** are the hard ceiling. In-app:

- Log every call to `usage`. Cost you can't see is cost you can't control.
- `AI_MONTHLY_BUDGET_USD` — check before each call; over budget → rules-only.
- Per-user daily caps (~20 suggestions, ~50 tagging jobs).
- `max_tokens` on every request. Never open-ended.
- 3 retries max, exponential backoff. Never retry 4xx.
- Prompt caching on the tagging system prompt — identical every call.

The realistic runaway risk is a retry loop in the worker, not normal usage.
Test the worker's failure path deliberately.
