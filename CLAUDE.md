# CLAUDE.md

Self-hosted digital wardrobe and lookbook. Photograph clothes, AI tags them, get
outfit suggestions by weather / occasion / season / mood / time of day. Save
outfits as "looks", log what you wore.

Household app, two users, one small VPS. Optimize for simplicity, never scale.

## Read these when relevant

| Working on… | Read first |
|---|---|
| Tagging, filtering, item attributes | `docs/taxonomy.md` |
| Migrations, any query touching user data | `docs/data-model.md` |
| Tagging jobs, suggestions, API calls | `docs/ai.md` |
| Login, uploads, the worker | `docs/backend.md` |
| Any UI, screens, PWA, capture | `docs/frontend.md` |
| Setting up locally, or deploying | `docs/deployment.md` |

Read the relevant file before writing code, not after.

## Constraints

| Constraint | Consequence |
|---|---|
| Hetzner CX23 — 2 vCPU, 4 GB, `linux/amd64` | SQLite only. No Redis, no Postgres. |
| Photos on local disk | Resize aggressively. No object storage SDKs. |
| Mobile-first | PWA, camera capture, cellular-friendly. |
| One maintainer, sporadic attention | Boring, obvious code. |

**Hard rule: `CGO_ENABLED=0`.** One static binary, `scp`-able, no runtime deps.
Disqualifies `mattn/go-sqlite3` and libvips-based image libraries.

The app should idle in tens of megabytes. If it doesn't, something is wrong.

## Stack

- **Go 1.22+** — stdlib `net/http` router (method+wildcard patterns). No chi/gin.
- `modernc.org/sqlite` — pure-Go driver, no cgo
- `golang.org/x/image` + `image/jpeg` — resizing
- `go:embed` — frontend and migrations baked into the binary
- **React + TypeScript + Vite**, TanStack Query, Tailwind. No Redux.
- Deploy: binary + systemd + Caddy. No Docker in production.

```
cmd/wardrobe/main.go
internal/{http,store,ai,images,weather,jobs}/
migrations/            NNNN_name.sql, embedded
web/                   React app
data/                  gitignored: wardrobe.db, photos/
```

**Layering:** `http` → `store`. Handlers never write SQL. `store` never imports
`http`. `ai` returns plain structs, knows nothing about HTTP.

## Code quality

### Definition of done

Compiles, `make test` passes, `make lint` clean, and exercised manually in the
browser. No commented-out code. No `TODO` without an issue number.

### Test where bugs hide

Not for coverage percentage. In priority order:

1. **Enum validators** — every invalid value rejected. Table-driven.
2. **Outfit pre-filter** — fixture wardrobe, assert correct candidates for given
   weather and occasion.
3. **Ownership checks** — user A cannot read, edit, or delete user B's items.
4. **Image pipeline** — EXIF stripped, orientation correct, sizes right.
5. **AI response parsing** — malformed JSON, out-of-vocab values, hallucinated
   ids. Recorded fixtures, never live API calls in tests.

Handlers get integration tests via `httptest`. Don't unit-test glue.

Items 3 and 5 are first because they fail *silently* — a leaked wardrobe throws
no error, and a hallucinated id renders as a broken card rather than a crash.

### Security

- Parameterized SQL only. No string concatenation, ever.
- Ownership filter in the `store` layer on every query touching user data.
- EXIF stripped from every uploaded image.
- Never log API keys, password hashes, session tokens, or full request bodies.
- Rate-limit auth endpoints and anything that triggers an API call.
- Validate and cap all user input: upload size, string and array lengths.

### Style

- Wrap errors: `fmt.Errorf("...: %w", err)`. Never `_ = err`.
- `context.Context` first param on anything doing I/O.
- `log/slog` for structured logs.
- **No premature abstraction.** No interface with one implementation — the `ai`
  interface is the deliberate exception. No generics without a second caller.
- Frontend conventions live in `docs/frontend.md`. Read it before any UI work.

### Degrade, don't crash

Weather down → suggest anyway. AI down → rules-only. Budget exceeded →
rules-only. The wardrobe must always be browsable.

## Commands

```bash
make dev       # Go backend + Vite dev server
make build     # frontend, embed, compile for host
make release   # GOOS=linux GOARCH=amd64 CGO_ENABLED=0
make test      # go test ./... (no frontend tests yet)
make lint      # golangci-lint + oxlint
```

Default `AI_PROVIDER=mock` locally — never burn API credits during development.

## Issue tracking

GitHub Issues via the official GitHub MCP server (remote URL + OAuth). Ignore
the deprecated `@modelcontextprotocol/server-github` npm package.

One issue per build-order phase, broken into checklist items. Reference them in
commits (`closes #12`). Check open issues before starting; verify against the
definition of done before closing.

No AI attribution in commits or PRs: no `Co-Authored-By: Claude` trailer, no
"Generated with Claude Code" footer.

## Build order

Each phase usable before the next starts.

1. Skeleton: server, SQLite, migrations, embedded React, `/healthz`
2. Auth + CLI user creation
3. Item CRUD, photo upload, image pipeline
4. Manual tagging UI against the closed vocabularies
5. Job queue + AI vision tagging
6. Weather
7. **Rules-based outfit pre-filter** — works with zero AI
8. LLM suggestions layered on top
9. Looks — save, browse, edit
10. Wear logging + history
11. PWA manifest, service worker, install prompt

Phase 7 before 8 is deliberate: the deterministic filter is both the foundation
and the permanent fallback.

## Non-goals

Social features. Sharing. E-commerce or shopping suggestions. Native mobile
apps. Background removal. Multi-tenancy. Billing. Docker in
production. Kubernetes. Local LLM hosting — a 4 GB VPS can't, and the API costs
a few dollars a month.

## When in doubt

- **Ask before adding a dependency.** The list above is short on purpose.
- **Ask before changing the schema** once real data exists.
- Prefer boring code. This will be maintained sporadically by one person who
  will have forgotten how it works.
- If a feature needs >2 GB RAM, a second process, or horizontal scaling, it's
  the wrong design for this project.
