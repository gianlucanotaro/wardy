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

With no signup form, the only attack surface is credential guessing against two
known accounts — which rate limiting handles.

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

**HEIC gotcha:** iPhones shoot HEIC. Safari usually converts on
`<input capture>`, but library picks can yield HEIC and pure-Go decoding isn't
viable. Detect server-side and return a clear error.

## Background jobs

`jobs` table + one worker goroutine polling every few seconds. No Redis.

Idempotent — the worker can restart mid-flight. Exponential backoff, max 3
attempts, persist `last_error`. Only job type in v1: `tag_item`.
