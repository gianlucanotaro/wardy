# CLAUDE.md

Project guidance for Claude Code. Read this before making changes.

---

## 1. What this is

A self-hosted **digital wardrobe and lookbook**. Users photograph their clothes,
an AI tags each garment with structured attributes, and the app suggests outfits
based on weather, occasion, season, mood, and time of day. Users save outfits
they like as "looks" and track what they actually wore.

**Household app, not a product.** A handful of users on one box. Optimize for
simplicity and low resource use, never for scale.

### Core loop

1. Photograph a garment on a phone → upload
2. Background worker calls a vision model → structured tags
3. User reviews/corrects tags (AI is a first draft, never the final word)
4. User asks for a suggestion → LLM composes outfits from the tagged wardrobe
5. User saves an outfit as a **look**, or logs that they wore it

---

## 2. Constraints that drive every decision

| Constraint | Consequence |
|---|---|
| Oracle Cloud Always Free, **2 OCPU / 12 GB RAM** | No Redis, no Postgres, no Elasticsearch. SQLite only. |
| **ARM64** (`aarch64`) target | Every dependency must be ARM-clean. Prefer pure Go. |
| Photos on **local disk**, 200 GB block volume | Must resize aggressively; no object storage SDKs. |
| Mobile-first usage | PWA, camera capture, works over cellular. |
| Free tier can be reclaimed if idle | Keep a lightweight cron/heartbeat. |

**Hard rule: `CGO_ENABLED=0`.** The whole point is a single static ARM64 binary
you can `scp` to the box. Any dependency requiring cgo is disqualified — this
specifically rules out `mattn/go-sqlite3` and `libvips`-based image libraries.

**Development happens on an Apple Silicon Mac (`darwin/arm64`) and deploys to
`linux/arm64`.** Same architecture on both ends, so nothing is discovered late.
Only `GOOS` changes between local and production builds.

---

## 3. Stack

**Backend** — Go 1.22+
- `net/http` stdlib router (Go 1.22 method+wildcard patterns). No chi/gin/echo.
- `modernc.org/sqlite` — pure-Go SQLite driver, no cgo
- `golang.org/x/image` + stdlib `image/jpeg` for resizing
- `go:embed` for the built frontend and SQL migrations

**Frontend** — React + TypeScript + Vite
- TanStack Query for server state. No Redux.
- Tailwind CSS
- Built to `web/dist`, embedded into the Go binary

**Deployment** — one binary + one SQLite file + one photos directory. Caddy in
front for TLS. Docker is optional; do not require it.

---

## 4. Layout

```
cmd/wardrobe/main.go       entrypoint
internal/
  http/                    handlers, middleware, routing
  store/                   SQLite queries; ALL SQL lives here
  ai/                      vision tagging + outfit suggestion
  images/                  resize, thumbnail, EXIF orientation
  weather/                 Open-Meteo client
  jobs/                    background worker
migrations/                NNNN_name.sql, embedded
web/                       React app
data/                      gitignored: wardrobe.db, photos/
```

**Layering rule:** `http` → `store`. Handlers never write SQL; `store` never
imports `http`. `ai` returns plain structs and knows nothing about HTTP.

---

## 5. Data model

SQLite, WAL mode, `foreign_keys=ON`, `busy_timeout=5000`.

```
users        id, email, password_hash, display_name, created_at
sessions     token, user_id, expires_at

items        id, user_id, name, category, subcategory,
             color_primary, color_secondary, pattern, material,
             formality, warmth, fit, brand, notes,
             seasons (JSON array), occasions (JSON array),
             photo_path, thumb_path,
             ai_tagged_at, user_edited_at, archived, created_at

looks        id, user_id, name, notes, cover_item_id, created_at
look_items   look_id, item_id, position

wears        id, user_id, look_id (nullable), worn_on (date),
             occasion, weather_summary, notes, created_at
wear_items   wear_id, item_id

jobs         id, type, payload (JSON), status, attempts,
             last_error, run_after, created_at
```

### Wardrobe separation

Every user has their **own wardrobe**. `items.user_id` is mandatory and every
query filters on it. Users cannot see each other's items — enforce this in the
`store` layer, not just in handlers, so a missed check can't leak data.

`looks` are also per-user. There is no sharing between accounts in v1.

### Wear history

Logging a wear is the only tracking feature in scope. It powers "you haven't
worn this in 8 months" and "don't suggest what I wore yesterday." Deliberately
**not** in scope: laundry state, cost-per-wear, seasonal storage.

---

## 6. Tagging taxonomy — closed vocabularies

This is the most important section. Free-text AI tags make filtering useless.
Every attribute below is a **fixed enum**, defined once in Go and mirrored in
TypeScript. The vision model must be constrained to these values, and any
response containing an out-of-vocabulary value is rejected and retried once.

```
category      top | bottom | dress | outerwear | shoes | bag | accessory
subcategory   constrained per category (t-shirt, button-down, knit, jeans,
              chinos, shorts, skirt, blazer, coat, sneakers, boots, ...)
pattern       solid | striped | checked | floral | graphic | textured | other
material      cotton | linen | wool | cashmere | denim | leather | silk |
              synthetic | blend | unknown
fit           slim | regular | relaxed | oversized
color         fixed ~20-value palette (do NOT let the model invent
              "sage mist" — map to the nearest palette entry)
seasons       array of: spring | summer | autumn | winter
occasions     array of: work | casual | formal | date | sport | outdoor | home
```

Two **numeric scales** do the heavy lifting for matching:

```
formality  1 = loungewear ... 5 = black tie
warmth     1 = hot weather only ... 5 = deep winter
```

Weather matching is mostly arithmetic on `warmth`; occasion matching is mostly
arithmetic on `formality`. Keep it that way — cheap, instant, debuggable.

---

## 7. AI integration

Two distinct jobs, two different models. Both via the Anthropic Messages API.

### 7a. Vision tagging — `claude-haiku-4-5-20251001`

Classification and extraction is exactly Haiku's sweet spot, and it's the
cheapest current model at $1/$5 per million tokens. Tagging a ~200-item wardrobe
costs well under a dollar **once** — this is not a recurring cost, so don't
over-engineer around it.

- Runs as a **background job**, never inline in the upload request. Upload
  returns immediately; the UI polls or shows a "tagging…" state.
- Send the *thumbnail*, not the original. ~800px longest edge is plenty and
  cuts token cost substantially.
- Request JSON only, no prose, no markdown fences. Strip fences defensively
  anyway.
- **Validate every field against the enums.** Reject and retry once, then mark
  the job failed and let the user tag manually.
- Store `ai_tagged_at`. When a user edits, set `user_edited_at` and never
  overwrite their values with a re-run.

### 7b. Outfit suggestion — `claude-sonnet-5`

This needs reasoning and a readable explanation, so it's worth the better model.

**Never send the whole wardrobe.** The pipeline is:

1. **Pre-filter in SQL** — season matches, `warmth` within range of the
   forecast, `formality` within ±1 of the occasion's target, not archived,
   not worn in the last N days.
2. **Cap at ~60 candidates.** If more survive, sample across categories so
   every slot has options.
3. Send a compact JSON array — `id`, and only the attributes that matter.
   No photo paths, no timestamps, no prose.
4. Ask for 2–3 complete outfits as JSON: arrays of item IDs plus a one-sentence
   rationale each.
5. **Validate every returned ID** exists, belongs to this user, and was in the
   candidate set. Models hallucinate IDs. Drop invalid outfits rather than
   showing a broken card.

Mood and time of day go in the prompt as context, not as filters — they shape
the rationale and the ranking, not the candidate set.

**If the API is unreachable, degrade to a rules-only suggestion** built from the
same pre-filter. The app must stay usable without network access to the API.

### 7c. Billing — read this before wiring anything up

**A Claude Pro/Max subscription does not include API access.** They are separate
billing contracts with no shared balance. The subscription covers the Claude
apps and subscription-authenticated Claude Code; this app's runtime calls need
an API key from console.anthropic.com with prepaid credits.

Do **not** attempt to route this app's backend through subscription
credentials — headless Claude Code, session-token scraping, or similar. It's
outside the intended use of subscription auth, it breaks on every credential
rotation, and it ends up billing an API key regardless.

Practical split for this project:
- **Claude Code on the subscription** → building the app
- **API key with credits** → the app's own tagging and suggestion calls

Expected spend is low: ~$0.60 to tag a 200-item wardrobe (one-time), and
~$0.02 per outfit suggestion. Realistically $1–3/month for a two-person
household.

### API key handling

`ANTHROPIC_API_KEY` from environment only. Never in the database, never in a
config file in the repo, never sent to the frontend. All model calls originate
from the Go backend.

Log token usage per call to a `usage` table so cost is observable rather than a
surprise. Enable prompt caching on the tagging system prompt — it's identical
across every item and cache reads cost a fraction of standard input.

### 7e. Budget guards — build these in

Console prepaid credits with **auto-reload off** are the hard ceiling; the app
should never be the reason you hit it. Defense in depth:

- **`usage` table** — log model, input/output tokens, and estimated cost for
  every call. Cost you can't see is cost you can't control.
- **Monthly budget check** — a configurable `AI_MONTHLY_BUDGET_USD`. Before any
  API call, sum the current month's estimated spend; if over budget, skip the
  call and serve the rules-based suggestion instead. Degrade, never fail.
- **Per-user daily caps** — e.g. 20 suggestions and 50 tagging jobs per user per
  day. Prevents both accidents and a stuck client retrying in a loop.
- **`max_tokens` on every request.** Tagging needs a few hundred; suggestions a
  few thousand. Never leave it open-ended.
- **Retry ceiling** — 3 attempts with exponential backoff, already specified in
  §11. Never retry on 4xx; those won't succeed on a second try.
- **Kill switch** — `AI_ENABLED=false` disables all outbound model calls and
  falls back to rules-only. The app must remain fully usable in this state.

The realistic runaway risk here is a retry loop in the background worker, not
normal usage. Test the worker's failure path deliberately.

### 7f. Optional: local model fallback

Structure `internal/ai` behind an interface with two implementations —
`anthropic` and `ollama` — selected by config. This is not v1 work, but design
for it now so it isn't a rewrite later.

If cost becomes a concern, the sensible split is **cloud for vision tagging,
local for outfit suggestions**. Vision needs the better model and is a one-time
cost per garment; text-only outfit ranking is recurring and tolerant of a weaker
model, especially since the SQL pre-filter already enforces the hard
constraints.

Budget realistically: the Oracle free tier is **2 OCPU / 12 GB** after the June
2026 cut, so expect a quantized 7–8B text model at single-digit tokens per
second. Fine for a few suggestions a day; not acceptable for anything
interactive. Local *vision* models are not viable on this hardware — don't try.

---

## 8. Weather

**Open-Meteo** — free, no API key, no signup, generous limits. Do not add a
provider that requires registration.

- Cache forecasts in-memory, keyed by rounded lat/lon, TTL 1 hour.
- Users store a home location in settings. Don't demand browser geolocation.
- Suggestions need: temp high/low, precipitation probability, wind, condition.
- If weather fetch fails, suggest anyway with a note — never block on it.

---

## 9. Auth

Deliberately minimal, but not sloppy.

- Email + password, hashed with **argon2id** (`golang.org/x/crypto/argon2`).
- Opaque random session tokens in the DB. No JWT.
- Cookies: `HttpOnly`, `Secure`, `SameSite=Lax`.
- **No public signup.** Accounts are created by CLI flag:
  `./wardrobe create-user --email x@y.z`. This is a household app on the open
  internet; an open registration form is an invitation.
- Rate-limit login attempts per IP and per email.

---

## 10. Image pipeline

On upload:
1. Validate content type and size cap (~15 MB).
2. Read EXIF orientation and **rotate the pixels**, then strip all EXIF.
   Phone photos carry GPS coordinates — stripping this is mandatory.
3. Write two derivatives: display (~1600px longest edge) and thumb (~400px).
4. Keep the original only if disk allows; it's rarely needed.
5. Store under `data/photos/<user_id>/<uuid>.jpg`. Never trust client filenames.
6. Serve through an authenticated handler that checks ownership — **never**
   expose the photos directory as a static file mount.

**Known gotcha — HEIC.** iPhones shoot HEIC. Safari usually converts to JPEG
when using `<input type="file" capture>`, but picking from the photo library can
yield HEIC, and pure-Go HEIC decoding is not viable. Detect HEIC server-side and
return a clear error; convert client-side in the browser before upload if this
turns out to be common in practice.

---

## 11. Background jobs

A `jobs` table plus one worker goroutine polling every few seconds. No Redis, no
external queue — 12 GB is plenty, but there's no reason to spend it.

- Exponential backoff, max 3 attempts, persist `last_error`.
- Jobs must be idempotent; the worker can restart mid-flight.
- Only job type in v1: `tag_item`.

---

## 12. Conventions

- Errors wrapped with `fmt.Errorf("...: %w", err)`. Never `_ = err`.
- `context.Context` as first parameter on anything doing I/O.
- `log/slog` for structured logs. **Never log API keys, password hashes, or
  session tokens.**
- Table-driven tests. Test the enum validators and the outfit pre-filter
  thoroughly — those are where bugs hide.
- SQL as parameterized queries. No string concatenation, ever.
- Migrations are append-only. Never edit a committed migration.

### Frontend

- Function components, hooks only.
- TanStack Query owns all server state; `useState` for local UI only.
- Mobile-first — design at 380px, then widen.
- Optimistic updates on tag edits; they should feel instant.

---

## 13. Commands

```bash
make dev          # Go backend + Vite dev server
make build        # build frontend, embed, compile for the host
make release      # build linux/arm64 for deployment
make test         # go test ./... + vitest
make lint         # golangci-lint + eslint

# production build
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o wardrobe ./cmd/wardrobe
```

---

## 14. Local development

Everything runs locally with no cloud dependencies.

```bash
cp .env.example .env    # AI_PROVIDER=mock to start
make dev                # Go :8080 + Vite :5173 (proxies /api)
./wardrobe create-user --email me@local
make seed               # optional: sample items with placeholder images
```

State lives in `./data/` — `wardrobe.db` and `photos/`. Delete the directory to
reset. Both are gitignored.

### Never burn credits in development

`AI_PROVIDER` accepts `anthropic`, `ollama`, or **`mock`**. The mock returns
fixture tags and fixture outfits instantly with zero network calls. **This is
the default in `.env.example`.**

You will iterate on the tagging review UI hundreds of times. None of those
iterations need a real model. Switch to `anthropic` only to validate prompt
changes, then switch back.

### Testing camera capture from a phone

`localhost` is a secure context, so the camera works in your desktop browser.
**Your laptop's LAN IP is not**, so `http://192.168.x.x:5173` will silently fail
on `getUserMedia` and the service worker won't register.

Use a tunnel for phone testing:

```bash
cloudflared tunnel --url http://localhost:8080
```

Do not waste an afternoon debugging "camera doesn't work on mobile" — it is
almost always this.

---

## 15. Deployment

Target: **Oracle Cloud Always Free**, Ubuntu 24.04 **aarch64**, Frankfurt
region (good capacity, low latency from Zürich).

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o wardrobe ./cmd/wardrobe
scp wardrobe user@host:/opt/wardrobe/
ssh user@host 'sudo systemctl restart wardrobe'
```

One static binary. No Docker required, no runtime dependencies, no package
installs on the box.

If the host ever changes to x86, this is a one-word edit (`GOARCH=amd64`) — the
`CGO_ENABLED=0` rule means no code change is needed. Don't design for it now.

### Runtime

- **systemd unit** with `Restart=always`, env vars in
  `/etc/wardrobe/env` (mode 0600 — it holds the API key).
- **Caddy** in front for automatic Let's Encrypt TLS. HTTPS is mandatory: the
  PWA's service worker and camera access both require a secure context.
- **Bind the app to `127.0.0.1`**, never `0.0.0.0`. Caddy is the only thing
  that should reach it.
- **Oracle's firewall is two layers.** Both the VCN Security List *and* the
  instance's local `iptables` must allow 80/443. OCI Ubuntu images ship with
  restrictive local rules — this trips up nearly everyone.
- **Idle reclamation:** Always Free instances can be reclaimed when idle. The
  health cron below is not optional.

### Backups

The whole application state is one SQLite file plus one photos directory.

- Nightly `sqlite3 wardrobe.db ".backup"` — never copy a live DB file directly.
- Photos are the irreplaceable part. Sync them off-box; a 200 GB volume on a
  reclaimable free instance is emphatically not a backup.
- Test a restore once. An untested backup is a guess.

### Monitoring

A cron hitting `/healthz` every 10 minutes serves double duty: keeps the
instance from looking idle to Oracle, and tells you when the app is down.

---

## 16. Build order

Do not jump ahead. Each phase should be usable before the next starts.

1. Skeleton: Go server, SQLite, migrations, embedded React, health check
2. Auth + CLI user creation
3. Item CRUD with photo upload and the full image pipeline
4. Manual tagging UI against the closed vocabularies
5. Job queue + AI vision tagging
6. Weather integration
7. Rules-based outfit pre-filter (works with no AI at all)
8. LLM outfit suggestion layered on top of the pre-filter
9. Looks — save, browse, edit
10. Wear logging + history views
11. PWA manifest, service worker, install prompt

Phase 7 before 8 is intentional: the deterministic filter is the foundation and
the permanent fallback. Get it right first.

---

## 17. Non-goals

Do not build these without being asked:

- Social features, sharing, public profiles
- E-commerce, shopping suggestions, affiliate links
- Mobile native apps — the PWA is the mobile app
- Background removal / virtual try-on
- Multi-tenancy, billing, admin dashboards
- Kubernetes, microservices, message brokers

---

## 18. When in doubt

- **Ask before adding a dependency.** The dependency list above is short on
  purpose.
- **Ask before changing the schema** once there's real data in it.
- Prefer boring, obvious code. This app will be maintained sporadically by one
  person who will have forgotten how it works.
- If a feature requires more than 12 GB of RAM or a second server process,
  it's the wrong design for this project.
