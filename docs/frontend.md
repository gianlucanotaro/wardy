# Frontend

Read before building any UI.

React + TypeScript + Vite, TanStack Query, Tailwind. Function components and
hooks only. Built to `web/dist`, embedded via `go:embed`.

Mobile-first: design at 380px, then widen. Most sessions happen on a phone while
getting dressed.

## Screens

Proposed set — confirm before building beyond phase 3.

| Route | Purpose |
|---|---|
| `/` | Wardrobe grid. Filter by category, season, formality, color. |
| `/items/new` | Capture or pick a photo, upload, land on review. |
| `/items/:id` | Detail and tag editing. |
| `/suggest` | Pick occasion + mood → outfit cards. |
| `/looks` | Saved outfits. |
| `/looks/:id` | Look detail, edit, log as worn. |
| `/history` | Wear log, "not worn in a while". |
| `/settings` | Home location, account. |

## Photo capture

Use a plain file input. Do **not** reach for `getUserMedia` — the native picker
handles orientation, compression, and permissions correctly, and `getUserMedia`
gives you a worse camera than the system one.

```html
<input type="file" accept="image/*" />
```

**Deliberately omit the `capture` attribute.** Adding `capture="environment"`
forces the camera and removes the option to pick an existing photo, which is how
most people will add clothes they already photographed. The tradeoff is that
library picks are more likely to be HEIC — handle the error path (see
`docs/backend.md`) rather than removing the choice.

## Upload and tagging flow

Tagging is a background job, so the UI must handle a pending state:

1. Upload returns immediately with the item id and `status: pending`.
2. Navigate straight to the review screen — don't block on a spinner.
3. Poll with TanStack Query `refetchInterval` while pending, stop when tagged
   or failed.
4. On failure, show the empty tag form. Never a dead end.

Show the photo immediately while tags load. The user should see progress, not a
blank screen.

## Tag review — the most important screen

This is where AI output becomes trustworthy data. It gets used on every single
item, so friction here compounds.

- **Every field is a picker, never a text input.** Segmented controls or selects
  bound to the enums in `docs/taxonomy.md`. A free-text field anywhere in this
  form defeats the entire taxonomy.
- `formality` and `warmth` are 1–5 ranges on segmented controls with worded
  endpoints ("loungewear" … "black tie"), not bare numbers — tap two steps to
  set a range.
- Mark AI-suggested values visually so the user knows what to double-check.
  Material is the least reliable — draw the eye there.
- Optimistic updates. Edits must feel instant; the round trip is irrelevant.
- One-tap save. This screen will be used a hundred times in a week.

## Outfit suggestions

Each outfit renders as a card: item thumbnails in a row, the model's
one-sentence rationale, and actions to save as a look or log as worn.

Handle three states explicitly:

- **Normal** — 2–3 cards.
- **Rules-only fallback** — AI unavailable or over budget. Show outfits without
  rationale text, with a quiet note. Do not show an error.
- **Nothing matches** — the pre-filter returned too little. Say which constraint
  is too tight rather than showing an empty screen.

## Images and layout

- Thumbnails in grids, display size on detail. Never the original.
- **Reserve aspect ratio on every image slot.** A wardrobe grid that reflows as
  photos load is the fastest way to make the app feel broken.
- `loading="lazy"` below the fold.
- Photos come from an authenticated endpoint — plain `<img src>` works since
  cookies are sent, but handle the 401 case on session expiry.

## PWA

`vite-plugin-pwa`. Manifest with a maskable icon, `display: standalone`.

**Cache the app shell only. Never cache API responses.** This is authenticated,
per-user data — a service worker serving another session's cached wardrobe is a
real failure mode. Network-first for everything under `/api`.

Offline target is modest: the shell loads and shows a clear offline message.
Browsing the wardrobe offline is a non-goal.

iOS specifics, since that's the likely primary device:

- No `beforeinstallprompt` on iOS. Add a short "Add to Home Screen" hint in
  settings rather than relying on a prompt that never fires.
- Handle safe-area insets (`env(safe-area-inset-*)`) or content sits under the
  notch and home indicator in standalone mode.
- Test in standalone mode specifically — it behaves differently from Safari.

## State

TanStack Query owns all server state. `useState` for local UI only. No Redux,
no global store.

Query keys mirror the resource path: `['items']`, `['items', id]`,
`['suggestions', occasion, mood]`. Invalidate `['items']` after any tag edit.

## Baseline quality

- Touch targets 44px minimum.
- Every input has a label. Icon-only buttons get `aria-label`.
- Visible focus states — don't strip outlines.
- Respect `prefers-reduced-motion`.
- Loading states for anything over ~300ms; skeletons over spinners in grids.
