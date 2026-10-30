# Toto

Self-hosted media library: Go API, Vue front-end, PostgreSQL.

## Development

Only Docker (with Compose) is required on the host.

```sh
cp .env.example .env   # set UID/GID to `id -u` / `id -g` if they are not 1000
cp compose.override.example.yaml compose.override.yaml   # list your media folders (read-only)
docker compose up
```

- Front-end: http://localhost:5173 (hot reload, proxies `/api` to the API)
- API: http://localhost:8080 (hot reload with air, migrations applied at startup)
- PostgreSQL: `localhost:5432`, user/password/database `toto`

On first launch the front-end asks for the administrator account.

### Everyday commands

```sh
docker compose exec api go test ./...         # API unit + integration tests (uses the toto_test database)
docker compose exec api go vet ./...
docker compose run --rm sqlc generate          # after editing api/db/queries or api/db/migrations
docker compose exec web npm test               # front-end tests
docker compose exec web npm run typecheck
docker compose exec web npm install <package>  # add a front-end dependency
```

## Layout

- `api/` — Go API (chi, pgx, sqlc, goose). Migrations in `api/db/migrations`, SQL queries in `api/db/queries`.
- `web/` — Vue 3 + Vite + TypeScript + Pinia + Tailwind.
- `docs/openapi.yaml` — API contract, shared with future mobile clients.

## Production

```sh
cp .env.example .env   # set POSTGRES_PASSWORD
docker compose -f compose.prod.yaml up -d --build
```
