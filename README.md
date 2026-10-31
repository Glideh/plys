# Plys

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

## Installation

Plys runs from two published images, `glide/plys-api` and `glide/plys-web`, next to PostgreSQL. Only Docker with Compose is needed.

```sh
mkdir plys && cd plys
curl -fsSLO https://raw.githubusercontent.com/Glideh/plys/main/compose.prod.yaml
curl -fsSL https://raw.githubusercontent.com/Glideh/plys/main/.env.example -o .env
```

Edit `.env`: set `POSTGRES_PASSWORD` (any long random value) and `MEDIA_DIR` (the folder holding your videos, mounted read-only). Then:

```sh
docker compose -f compose.prod.yaml up -d
```

Open `http://<host>:8096` and create the administrator account. **Do it right away**: until then, the first visitor can claim the server. Add your libraries from the administration screen (everything below `MEDIA_DIR` is offered) and, for posters and synopses, a free [TMDB API key](https://www.themoviedb.org/settings/api) in the settings.

- **Update:** `docker compose -f compose.prod.yaml pull && docker compose -f compose.prod.yaml up -d`. Set `PLYS_VERSION=1.2.3` in `.env` to stay on a release instead of `latest`.
- **HTTPS:** behind a reverse proxy that terminates TLS, set `COOKIE_SECURE=true`. Over plain http, keep it `false`: browsers drop secure cookies there and every page reload would sign you out.
- **Hardware transcoding (Intel/AMD):** set `RENDER_GID` in `.env` (`stat -c %g /dev/dri/renderD128`) and add `-f compose.gpu.yaml` to the command. Without a GPU, transcoding uses the CPU; `MAX_TRANSCODES` (default 2) caps the number of simultaneous conversions.
- **Data:** the database lives in the `db-data` volume and the image cache in `api-cache`. Back up the database with `docker compose -f compose.prod.yaml exec db pg_dump -U plys plys > plys.sql`.

The API image is large (about 840 MB) because it ships ffmpeg and the GPU drivers.

## Publishing the images

Maintainers only. Log in to Docker Hub (`docker login`), commit your changes, then:

```sh
./release.sh 0.1.0     # builds both images and pushes 0.1.0, 0.1 and latest (plus the major tag from 1.0.0), after confirmation
./release.sh --local   # builds :dev into the local Docker only
NAMESPACE=myaccount ./release.sh 0.1.0   # publish under another Docker Hub account
```

Images are built for `linux/amd64` with `docker-bake.hcl`, with provenance and SBOM attestations. Users can run a local build with `PLYS_NAMESPACE=glide PLYS_VERSION=dev docker compose -f compose.prod.yaml up -d`.
