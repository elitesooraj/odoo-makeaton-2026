# Deployment

The host needs Docker with the Compose plugin. Nothing else: Go, Node and the
database client are not required.

## 1. Prepare the server

```bash
git clone git@github.com:elitesooraj/odoo-makeaton-2026.git
cd odoo-makeaton-2026
cp .env.example .env
```

Edit `.env`. At minimum:

| Variable | Production value |
| --- | --- |
| `APP_ENV` | `production` |
| `POSTGRES_PASSWORD` | a long random secret |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | the bootstrap admin, strong password |
| `CORS_ALLOWED_ORIGINS` | `https://your-domain` |
| `APP_BASE_URL` | `https://your-domain` (reset links use this) |
| `COOKIE_SECURE` | `true` (the prod override also forces it) |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` | real mail provider |
| `SMTP_FROM` | the verified sender address |
| `API_PORT` | usually `8080` behind a reverse proxy |
| `FRONTEND_PORT` | usually `80` behind a reverse proxy |

`docker compose -f docker-compose.yml -f docker-compose.prod.yml config`
prints the resolved configuration without starting anything.

## 2. Start

```bash
make up-prod          # build images, migrate, start api + frontend
make seed-prod        # once: create admin and finance staff
make ps
make logs
```

Or in one step:

```bash
make bootstrap-prod
```

`make up-prod` does **not** seed. In development the seed runs as part of every
`docker compose up` (idempotent, never overwrites a password); the production
override re-profiles it behind `tools` so account creation stays a deliberate,
explicit operation on a server.

The boot order is enforced by Compose:

```text
postgres (healthy) -> migrate (exit 0) -> api
                                 \-> seed (exit 0)
```

## 3. TLS and reverse proxy

Put Caddy, Nginx or a cloud load balancer in front of `API_PORT` and
`FRONTEND_PORT`. The API sets an `HttpOnly`, `SameSite=Lax` cookie with
`Secure` enabled in production, so plain HTTP will not keep a session.

Example Caddyfile:

```caddy
app.example.com {
    handle /api/* {
        reverse_proxy 127.0.0.1:8080
    }
    handle /healthz {
        reverse_proxy 127.0.0.1:8080
    }
    handle {
        reverse_proxy 127.0.0.1:80
    }
}
```

Set `CORS_ALLOWED_ORIGINS` and `APP_BASE_URL` to `https://app.example.com`.

## 4. Backups

The only state is the `postgres_data` volume.

```bash
docker compose exec -T postgres \
  pg_dump -U brightloop brightloop | gzip > backup-$(date +%F).sql.gz
```

Test a restore on a non-production database before relying on it.

## 5. Upgrades

```bash
git pull
make up-prod          # rebuilds, runs new migrations, recreates changed services
```

Migrations run before the new API starts. Applied migrations are immutable, so
rolling back means deploying the previous commit; the database is not reverted
automatically.

## 6. Hardening checklist

- [ ] `POSTGRES_PASSWORD` changed from the example.
- [ ] `ADMIN_PASSWORD` changed from the example.
- [ ] `COOKIE_SECURE=true` and the site is served over HTTPS.
- [ ] `CORS_ALLOWED_ORIGINS` lists only the real frontend origin.
- [ ] Real SMTP configured; send one password-reset email as a smoke test.
- [ ] Postgres and Mailpit are not reachable from outside the host. Both bind
      to `127.0.0.1`; do not change that.
- [ ] Host firewall allows only 80/443 (and SSH).
- [ ] Docker log rotation is active (`max-size` in the Compose files).
- [ ] A backup runs on a schedule and has been restored once.
- [ ] `make config-prod` passes on the server.
- [ ] Images are pinned by tag or digest before a long-lived deployment.

## Notes

- In the base (development) file, only `test` (`tools` profile) and `frontend`
  (`frontend` profile) are profile-gated; everything else — postgres, migrate,
  seed, api, mailpit — starts with plain `docker compose up`.
- The production override re-profiles `mailpit` under `dev` and `seed` under
  `tools`, so `make up-prod` never starts the mail catcher and never seeds
  automatically.
- The API image is `brightloop-api:local` by default. CI can push the same image
  to a registry and set `API_IMAGE` instead of rebuilding on the server.
- `make clean` deletes the database volume. There is no confirmation prompt
  beyond a three-second pause.
