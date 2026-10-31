# Plys

Plys is a self-hosted media library for personal video collections.

- **Libraries** of movies, series and other videos, built by scanning media folders (never modified).
- **Metadata** from TMDB: posters, synopses, cast; matches can be corrected by hand.
- **Playback in any browser**, with audio and subtitle selection; whatever the browser cannot play is converted on the fly, on the GPU when there is one.
- **Versions and parts**: several qualities or editions of a movie, and multi-part movies.
- **Progress**: resume where playback stopped, next episodes, watched marks, optional autoplay of the next episode.
- **Users**: an administrator, other accounts, and libraries restricted to chosen users.
- **Multilingual**: (French and English for now) interface, and a documented API (`docs/openapi.yaml`).

## Installation

Docker (with Docker Compose) is needed on the host.

### Quick start

Save this as `compose.yaml` in an empty folder and replace `/path/to/the/videos` with the folder holding the videos:

```yaml
# Smallest working setup. Set the videos folder below, then run: docker compose up -d
name: plys

services:
  db:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: plys
      POSTGRES_PASSWORD: plys
      POSTGRES_DB: plys
    volumes:
      - db-data:/var/lib/postgresql/data

  api:
    image: glide/plys-api:latest
    restart: unless-stopped
    environment:
      DATABASE_URL: postgres://plys:plys@db:5432/plys?sslmode=disable
      COOKIE_SECURE: "false"
    volumes:
      # The videos, read-only. Change the left part.
      - /path/to/the/videos:/media:ro
      - api-cache:/cache
    depends_on:
      - db

  web:
    image: glide/plys-web:latest
    restart: unless-stopped
    ports:
      - "8096:8080"
    depends_on:
      - api

volumes:
  db-data:
  api-cache:
```

Then start it with `docker compose up -d` and open `http://<this machine>:8096`. The first visitor creates the administrator account, so do it right away.

This setup keeps the defaults simple: the database password is `plys` (the database port is not published, so only the containers of this project can reach it) and the containers are not hardened. The complete installation below adds a chosen password, hardening, options and hardware transcoding. To move from one to the other, keep `POSTGRES_PASSWORD=plys` in `.env`: PostgreSQL only applies the password at the first start.

### Complete installation

**1. Create a folder for Plys** and, inside it, a file named `compose.yaml` with the content below. It uses the published images, so there is nothing to build.

```yaml
# Plys, a self-hosted media library.
# Save this file as compose.yaml next to a .env file (see the README), then run: docker compose up -d
name: plys

services:
  # Stores users, libraries, metadata and watch progress.
  db:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: plys
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD in .env}
      POSTGRES_DB: plys
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U plys -d plys"]
      interval: 5s
      timeout: 3s
      retries: 20

  # Scans the videos, serves the API and converts what a browser cannot play (ffmpeg).
  api:
    image: glide/plys-api:${PLYS_VERSION:-latest}
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    environment:
      DATABASE_URL: postgres://plys:${POSTGRES_PASSWORD}@db:5432/plys?sslmode=disable
      # Keep false when browsing over plain http (home network).
      # Set true if a reverse proxy serves Plys over HTTPS, otherwise browsers drop the session cookie.
      COOKIE_SECURE: ${COOKIE_SECURE:-false}
      # How many videos may be converted at the same time.
      MAX_TRANSCODES: ${MAX_TRANSCODES:-2}
      LOG_LEVEL: ${LOG_LEVEL:-info}
    volumes:
      # The videos, read-only: Plys never modifies them. Every folder below is offered when creating a library.
      - ${MEDIA_DIR:?Set MEDIA_DIR in .env}:/media:ro
      # Converted video segments, subtitles and downloaded posters.
      - api-cache:/cache
    # Hardware transcoding (Intel or AMD GPU): uncomment these four lines and set RENDER_GID in .env
    # (stat -c %g /dev/dri/renderD128). Without a GPU, conversions run on the CPU.
    # devices:
    #   - /dev/dri/renderD128:/dev/dri/renderD128
    # group_add:
    #   - "${RENDER_GID}"
    depends_on:
      db:
        condition: service_healthy

  # The web interface; also forwards /api to the API. This is the only service opened in a browser.
  web:
    image: glide/plys-web:${PLYS_VERSION:-latest}
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    tmpfs:
      - /tmp
    ports:
      # Plys is then reachable at http://<this machine>:8096
      - "${HTTP_PORT:-8096}:8080"
    depends_on:
      - api

volumes:
  db-data:
  api-cache:
```

**2. Create a `.env` file** next to it:

```sh
# A long random password for the database; it never has to be typed.
POSTGRES_PASSWORD=choose-a-long-random-password
# The folder holding the videos on this machine.
MEDIA_DIR=/path/to/the/videos
```

**3. Start it:**

```sh
docker compose up -d
```

**4. Open `http://<this machine>:8096`** and create the administrator account. Do it right away: until then, the first visitor can claim the server. Then add the libraries from the administration screen, and paste a free [TMDB API key](https://www.themoviedb.org/settings/api) in the settings to get posters and synopses.

### Options

Set these in `.env` when needed (complete installation):

| Variable | Default | Purpose |
|---|---|---|
| `HTTP_PORT` | `8096` | Port of the web interface |
| `COOKIE_SECURE` | `false` | `true` behind an HTTPS reverse proxy; keep `false` over plain http |
| `PLYS_VERSION` | `latest` | Image version, e.g. `0.1.0` to stay on a release |
| `MAX_TRANSCODES` | `2` | Videos converted at the same time |
| `RENDER_GID` | | Group of the GPU render node, for hardware transcoding (see the comment in the compose file) |

### Everyday operations

```sh
docker compose pull && docker compose up -d     # update to the newest image of PLYS_VERSION
docker compose logs -f api                      # follow the server logs
docker compose exec db pg_dump -U plys plys > plys-backup.sql   # back up the database
```

Data lives in two Docker volumes: `db-data` (database) and `api-cache` (converted segments, posters; can be rebuilt). The videos are never modified.

The API image is large (about 840 MB) because it ships ffmpeg and the GPU drivers. Images are published for `linux/amd64`.

## Development

Only Docker (with Compose) is required on the host.

```sh
cp .env.example .env   # set UID/GID to `id -u` / `id -g` if they are not 1000
cp compose.override.example.yaml compose.override.yaml   # list the media folders (read-only)
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

### Layout

- `api/` — Go API (chi, pgx, sqlc, goose). Migrations in `api/db/migrations`, SQL queries in `api/db/queries`.
- `web/` — Vue 3 + Vite + TypeScript + Pinia + Tailwind.
- `docs/openapi.yaml` — API contract, shared with future mobile clients.
- `compose.minimal.yaml` and `compose.prod.yaml` — the two end-user compose files shown above; `release.sh` checks that the README copies are identical.

## Publishing the images

Maintainers only. Log in to Docker Hub (`docker login`), commit the changes, then:

```sh
./release.sh 0.1.0     # builds both images and pushes 0.1.0, 0.1 and latest (plus the major tag from 1.0.0), after confirmation
./release.sh --local   # builds :dev into the local Docker only
NAMESPACE=myaccount ./release.sh 0.1.0   # publish under another Docker Hub account
```

Images are built for `linux/amd64` with `docker-bake.hcl`, with provenance and SBOM attestations. To try a local build, run `PLYS_VERSION=dev docker compose -f compose.prod.yaml up -d` with a `.env` as above.

## License

MIT, see `LICENSE`. This product uses the TMDB API but is not endorsed or certified by TMDB.
