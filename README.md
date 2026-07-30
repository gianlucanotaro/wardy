# Wardy

A self-hosted digital wardrobe and lookbook. Photograph your clothes, let AI
tag them, and get outfit suggestions by weather, occasion, season, mood, or
time of day. Save outfits as "looks" and log what you actually wore.

Household app for two users on one small VPS — built for simplicity, not scale.

## Status

Pre-code. The schema, taxonomy, and conventions are settled (see `docs/`); the
implementation hasn't started. Follow progress in the
[phase issues](https://github.com/gianlucanotaro/wardy/issues) — one issue per
build-order phase, checked off as work lands.

## Stack

- **Backend** — Go 1.22+, stdlib `net/http`, `modernc.org/sqlite` (pure Go,
  `CGO_ENABLED=0`), `go:embed` for migrations and the built frontend.
- **Frontend** — React + TypeScript + Vite, TanStack Query, Tailwind.
- **Deploy** — one static binary + systemd + Caddy, on a Hetzner CX23. No
  Docker in production.

## Getting started

```bash
cp .env.example .env    # AI_PROVIDER=mock by default — no API key needed
make dev                # Go :8080 + Vite :5173
./wardrobe create-user --email me@local
```

There's no public signup — accounts are created via the CLI flag above.

## Docs

| Working on… | Read first |
|---|---|
| Tagging, filtering, item attributes | [`docs/taxonomy.md`](docs/taxonomy.md) |
| Migrations, any query touching user data | [`docs/data-model.md`](docs/data-model.md) |
| Tagging jobs, suggestions, API calls | [`docs/ai.md`](docs/ai.md) |
| Login, uploads, the worker | [`docs/backend.md`](docs/backend.md) |
| Any UI, screens, PWA, capture | [`docs/frontend.md`](docs/frontend.md) |
| Setting up locally, or deploying | [`docs/deployment.md`](docs/deployment.md) |

Full conventions, constraints, and build order live in [`CLAUDE.md`](CLAUDE.md).

## Non-goals

Social features, sharing, e-commerce, native mobile apps, background
removal/virtual try-on, multi-tenancy, billing, Kubernetes, local LLM hosting.
