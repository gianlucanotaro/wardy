# Wardy

A self-hosted digital wardrobe and lookbook. Photograph your clothes, let AI
tag them, and get outfit suggestions by weather, occasion, season, mood, or
time of day. Save outfits as "looks" and log what you actually wore.

Household app for two users on one small VPS — built for simplicity, not scale.

## Status

In daily use by its two users. All build-order phases are done; new work is
tracked in [issues](https://github.com/gianlucanotaro/wardy/issues).

## Features

- **Today** — an outfit for today's weather (where you are, or home), occasion
  and mood. Swipe for the next one; save it as a look or log it as worn.
- **Wardrobe** — photo grid with structured search (`color:`, `category:`… as
  chips). New photos are tagged by AI in the background, then reviewed with
  pickers only. Archive, delete, "never worn", add to Build.
- **Build** — fill the outfit slots by hand, or SUGGEST OUTFIT with occasion,
  mood and a request in your own words ("a fit with a red top"); every slot
  stays swappable.
- **Looks** and **Plan** — saved outfits, and a week to put them on.
- **History** — wear log and "not worn in a while".
- **Inspo** — photos of other people's fits, tagged by AI into a style profile
  that can steer suggestions, and a style radar comparing what you own, what
  you wear and what you save.
- **Degrades, never breaks** — AI down or over budget → rules-only
  suggestions; weather down → suggest anyway.
- **Login** — Authelia single sign-on, password as fallback, no public signup.
  Installable as a PWA.

## Stack

- **Backend** — Go 1.26, stdlib `net/http`, `modernc.org/sqlite` (pure Go,
  `CGO_ENABLED=0`), `go:embed` for migrations and the built frontend.
- **Frontend** — React + TypeScript + Vite, TanStack Query, Tailwind.
- **Deploy** — one static binary + systemd + Caddy on a 4 GB VPS. No Docker in
  production. Merging to `main` deploys itself within a few minutes (build,
  database backup, restart, health check); see
  [`docs/deployment.md`](docs/deployment.md).

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
