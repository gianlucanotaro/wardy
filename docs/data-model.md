# Data model

Read before writing migrations or any query touching user data.

SQLite, WAL mode, `foreign_keys=ON`, `busy_timeout=5000`.

```
users        id, email, password_hash, display_name, home_lat, home_lon,
             use_inspo, style_summary, style_summary_key, created_at
sessions     token, user_id, expires_at
items        id, user_id, name, category, subcategory, color_primary,
             color_secondary, pattern, material, formality_min,
             formality_max, warmth_min, warmth_max, fit,
             brand, notes, seasons(JSON), occasions(JSON), photo_path,
             thumb_path, ai_tagged_at, user_edited_at, archived, created_at
looks        id, user_id, name, notes, cover_item_id, created_at
look_items   look_id, item_id, position
wears        id, user_id, look_id?, worn_on, occasion, weather_summary, notes
wear_items   wear_id, item_id
inspo_fits   id, user_id, photo_path, thumb_path, styles(JSON), colors(JSON),
             pieces(JSON), fit, formality_min, formality_max, seasons(JSON),
             occasions(JSON), custom_tags(JSON), ai_tagged_at,
             user_edited_at, created_at
jobs         id, type, payload, status, attempts, last_error, run_after
usage        id, model, input_tokens, output_tokens, est_cost_usd, created_at
```

## Ownership — non-negotiable

Every user has a separate wardrobe. `user_id` is mandatory on every user-owned
row and filtered on every query.

**Enforce ownership in the `store` layer, not in handlers.** A missed handler
check must not be able to leak another user's wardrobe. This failure is silent —
it throws no error — so it needs a structural guarantee, not vigilance.

No sharing between accounts in v1.

## Scope

Wear history is the only tracking feature. Deliberately **not** in scope:
laundry state, cost-per-wear, seasonal storage.

## Migrations

Append-only. Never edit a committed migration. Embedded via `go:embed`.
