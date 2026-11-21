# Auth, images, jobs

Read before touching login, uploads, or the worker.

## Auth

- Email + password, **argon2id** (`golang.org/x/crypto/argon2`).
- Opaque random session tokens in the DB. No JWT.
- Cookies `HttpOnly`, `Secure`, `SameSite=Lax`.
- **No public signup.** `./wardrobe create-user --email x@y.z` only. This sits
  on the open internet; a registration form is an invitation.
- Password resets are CLI-only too: `./wardrobe set-password --email x@y.z`
  prompts for the new password and signs that account out everywhere.
- Rate-limit login by IP and by email.
- **Single sign-on (#51):** with SSO on, the login screen sends you to
  Authelia automatically (one login there covers every app); after logging
  out, or with `/?password`, it shows the password form and a "Sign in with
  Authelia" button instead. OpenID Connect (authorization code + PKCE) — `internal/http/oidc.go`,
  stdlib only. It signs in **existing** accounts only, matched by verified
  email, then by username (Wardy logins can be either); no match is refused,
  so SSO never creates accounts. Password login stays as the fallback when
  the provider is down. Configured by the `OIDC_*` env vars
  (`docs/deployment.md`).

With no signup form, the only attack surface is credential guessing against two
known accounts — which rate limiting handles. SSO adds no new way in: whoever
controls the provider's accounts (the server admin) already controls Wardy's.

## Image pipeline

1. Validate content type, cap ~15 MB.
2. Read EXIF orientation, **rotate pixels, strip all EXIF**. Phone photos carry
   GPS coordinates — stripping is mandatory.
3. Write display (~1600px) and thumb (~400px) derivatives.
4. Store at `data/photos/<user_id>/<uuid>.jpg`. Never trust client filenames.
5. Serve via an authenticated handler that checks ownership. **Never** expose
   the photos directory as a static mount.

Serve thumbs in grid views. A 4 MB original vs a 200 KB thumb is 20× the
bandwidth on cellular.

### Cut-outs

A photo can carry a cut-out: a **mask** next to the JPEG, made **on the
phone**, never on the server (a model takes hundreds of MB, which a 4 GB VPS
shared with other apps cannot spare, and the photo never has to leave the
phone for it).

- The phone runs ISNet (MIT) with onnxruntime-web in a worker
  (`web/src/cutoutWorker.ts`), then uploads the photo as usual plus an
  optional `mask` field (`PUT /api/items/{id}/mask` for an existing photo,
  `DELETE` to remove it).
- The mask is a **PNG whose alpha channel** is opaque where the garment is
  (a CSS mask reads alpha). The server re-encodes it black with that alpha, at
  most 1000px, ~20 KB, as `data/photos/<user_id>/<uuid>_mask.png`, recorded in
  `items.mask_path` (`''` for none). Every replacement gets a new name, so the
  URL (`?v=`) changes and browsers never show a stale one.
- When a mask is saved the server **measures the garment**: the box round
  everything the mask keeps, as fractions of the photo's width and height, plus
  the photo's width/height, stored as JSON in `items.mask_box`
  (`images.MeasureMask`). Cut-outs made before migration 0010 are measured
  when the server starts (`jobs.BackfillMaskBoxes`; a mask file it cannot read
  is logged and left). This is what the app uses to normalize sizes.
- The photo stays a plain JPEG: the original is never touched, and the AI
  tagger still sees it. The app shows the garment through the mask on its
  colour block (`ItemImg.tsx`), in its own colours.
- A mask is **user input**: only PNG is decoded, size is checked first, it must
  have the photo's shape (2%), and a mask that is all one value is refused
  (`images.ProcessMask`). A bad mask on **upload** is logged and ignored: the
  item is saved with its plain photo. On `PUT` it is a 400. Ownership is
  checked like the photo's.
- Model files are served by `GET /api/models/{name}` from `<data>/models/`
  (a fixed list of names, `models.go`); not installed means 404 and the app
  does not offer cut-outs. See `docs/deployment.md`.

**HEIC gotcha:** iPhones shoot HEIC. Safari usually converts on
`<input capture>`, but library picks can yield HEIC and pure-Go decoding isn't
viable. Detect server-side and return a clear error.

## Background jobs

`jobs` table + one worker goroutine polling every few seconds. No Redis.

Idempotent — the worker can restart mid-flight. Exponential backoff, max 3
attempts, persist `last_error`. Only job type in v1: `tag_item`.
