# Local development and deployment

Read at phase 1, and again when deploying.

## Local

```bash
cp .env.example .env    # AI_PROVIDER=mock
make dev                # Go :8080 + Vite :5173
./wardrobe create-user --email me@local
```

**Sample garments for test accounts.** `./wardrobe seed-samples --email devtest`
adds the ~30 tagged, illustrated garments in `samples/` (`garments.json` +
`photos/`), every category covered, to that account. It skips samples the
account still has (by name, tagged `sample`) and re-adds deleted ones, so it's
safe on every start: the dev server runs it for `devtest` via a systemd drop-in
(`wardrobe-dev-api.service.d/seed-samples.conf`). Never point it at a real
account on prod.

State in `./data/` — delete to reset. Gitignored.

**Never burn credits in development.** `AI_PROVIDER=mock` returns fixture tags
and outfits instantly, and is the default in `.env.example`. You will iterate on
the tagging UI hundreds of times; none of it needs a real call.

**Camera testing:** `localhost` is a secure context, your LAN IP is not — so
`getUserMedia` silently fails and the service worker won't register when testing
from a phone over WiFi. Use `cloudflared tunnel --url http://localhost:8080`.
Don't lose an afternoon to this.

## Production

Hetzner CX23 (2 vCPU, 4 GB, 40 GB NVMe), Ubuntu 24.04, Nuremberg or
Falkenstein. `linux/amd64`.

```bash
make release
scp wardrobe user@host:/opt/wardrobe/
ssh user@host 'sudo systemctl restart wardrobe'
```

- **systemd** unit, `Restart=always`, env in `/etc/wardrobe/env` (mode 0600 —
  it holds the API key).
- **Caddy** in front — automatic Let's Encrypt. HTTPS is mandatory: service
  workers and camera access both require a secure context.
- **Bind to `127.0.0.1`**, never `0.0.0.0`. Caddy is the only thing that should
  reach the app.
- Paths from `DB_PATH` and `PHOTOS_DIR`. Never hardcode.

No Docker in production. A Dockerfile is optional and dev-only; keep the door
open for a PaaS later, but don't let Docker into the dev loop.

## Backups

- Nightly `sqlite3 wardrobe.db ".backup"`. Never copy a live DB file.
- Photos are the irreplaceable part. Sync them off-box.
- Test a restore once. An untested backup is a guess.
- Cron `/healthz` every 10 minutes, alert on failure.
