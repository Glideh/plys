<p align="center"><img src="web/public/brand-mark.svg" alt="" width="128" /></p>

# Plys

Plys is a self-hosted media library for your own movies, series and other videos. Install it on a home server, then watch from a web browser or an Android TV.

- **Libraries** of movies, series and other videos, organized from the folder and file names.
- **Metadata** from TMDB: posters, synopses, cast and ratings, with manual editing and custom posters.
- **Search and filters** by genre, year, person, rating and more, plus a watchlist and resume where you stopped.
- **Playback** in the browser, with audio and subtitle selection, and online subtitles from OpenSubtitles.
- **Transcoding** on the CPU, or on an Intel or AMD GPU when there is one.
- **Several users**, invited with a link, a PIN code or a QR code.
- **Federation**: pair two Plys servers to browse and play each other's libraries.
- **Android TV app**, which updates itself from the GitHub releases.
- **Languages**: English, French and Brazilian Portuguese.

When the GPU and driver support it, HDR-to-SDR conversion uses hardware HEVC decoding, VAAPI tone mapping
when mastering metadata is available (otherwise Vulkan), and hardware H.264 encoding. Plys checks these
capabilities at startup and falls back to software tone mapping if a GPU path fails validation.

## Installation

You need a machine with [Docker](https://docs.docker.com/engine/install/) and Docker Compose.

### Quick start

Save this as `compose.yaml` in an empty folder and replace `/path/to/the/videos` with the folder holding the videos:

```yaml
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
      # The videos, must be mounted from this /media path.
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

Start it with `docker compose up -d` and open `http://<this machine>:8096`. The first visitor creates the administrator account, so do it right away, before the server can be reached from the internet.

This setup keeps things simple (the database password is `plys`). For a server you keep, use the complete installation below.

### Complete installation

It adds a database password of your choice, hardened containers, options and hardware transcoding.

**1. Create a folder for Plys** and, inside it, a `compose.yaml` file with the content below. Replace `/video/path` with the folder holding the videos.

```yaml
name: plys

services:
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
      # Paired servers must be reached over https by default; set false only on a trusted home network with plain http.
      PLYS_FEDERATION_REQUIRE_HTTPS: ${PLYS_FEDERATION_REQUIRE_HTTPS:-true}
      # How many videos may be converted at the same time.
      MAX_TRANSCODES: ${MAX_TRANSCODES:-2}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      # Proxies allowed to set X-Forwarded-For (CIDRs, comma-separated); empty means the compose network.
      TRUSTED_PROXIES: ${TRUSTED_PROXIES:-}
      # Checks GitHub once a day for a newer release, shown in the footer of the web interface.
      PLYS_UPDATE_CHECK: ${PLYS_UPDATE_CHECK:-true}
    volumes:
      # The videos, set it read-only and mount every folders needed into media which is the root for the library paths.
      - /video/path:/media:ro
      # Converted video segments, subtitles and downloaded posters.
      - api-cache:/cache
    # Hardware transcoding (Intel or AMD GPU): uncomment these lines and set RENDER_GID in .env
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
      - "8096:8080"
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
```

**3. Start it** with `docker compose up -d`.

**4. Open `http://<this machine>:8096`** and create the administrator account.

### First steps

1. **TMDB key**: the setup offers to paste a free [TMDB API key](https://www.themoviedb.org/settings/api), which brings posters, synopses and cast. It can also be added later in *Administration → Settings*.
2. **Libraries**: in *Administration → Libraries*, add a library for each folder (paths start at `/media`, the folder mounted in `compose.yaml`) and choose its kind and metadata language. The first scan starts right away.
3. **Subtitles** (optional): with an [OpenSubtitles](https://www.opensubtitles.com/consumers) API key in *Administration → Settings*, viewers can search and download subtitles from the player.
4. **Users**: in *Administration → Users*, create accounts or send invitation links.

To diagnose playback, open *Administration → Playback sessions*. For each session it compares the
source tracks, the client's reported capabilities, and the output, lists why audio or video is
transcoded, shows whether video decoding, tone mapping, subtitle burn-in, scaling, and encoding use
the CPU or GPU, and shows FFmpeg's current speed: below 1×, the server cannot keep up with playback.
It only lists sessions handled by this server and reports FFmpeg state, not whether the viewer is paused.

Files are easiest to recognize when named like `Movie Title (2020).mkv` and `Series Name/Season 01/Series Name S01E02.mkv`. A wrong match can be fixed from the title's page.

### Updating

```sh
docker compose pull
docker compose up -d
```

The database is migrated automatically at startup. To stay on a given release, set `PLYS_VERSION` in `.env`. A major version can break the compatibility between paired servers: their versions are shown in *Administration → Servers*.

### Options

Set these in `.env` when needed (complete installation):

| Variable | Default | Purpose |
|---|---|---|
| `COOKIE_SECURE` | `false` | `true` behind an HTTPS reverse proxy; keep `false` over plain http |
| `PLYS_FEDERATION_REQUIRE_HTTPS` | `true` | Refuses paired servers whose public URL is not `https://`; set `false` only for pairing over plain http on a trusted network |
| `PLYS_VERSION` | `latest` | Image version, e.g. `0.7.9` to stay on a release |
| `MAX_TRANSCODES` | `2` | Videos converted at the same time |
| `RENDER_GID` | | Group of the GPU render node, for hardware transcoding (see the comment in the compose file) |
| `TRUSTED_PROXIES` | compose network | Proxies allowed to set `X-Forwarded-For`, as comma-separated CIDRs or addresses (`none` trusts none) |
| `PLYS_UPDATE_CHECK` | `true` | Checks GitHub once a day for a newer release and shows it in the footer of the web interface; `false` disables it |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

## Android TV

The app runs on Android TV and Google TV, Android 8.0 or newer.

1. Download `app-release.apk` from the [latest release](https://github.com/Glideh/plys/releases/latest) (older releases use `app-debug.apk`).
2. Install it on the TV, for example with a file manager app or `adb install app-release.apk`; Android asks to allow apps from unknown sources.
3. Sign in with the server address, such as `http://192.168.1.10:8096`, and an account created on the web.

The app then checks for new releases by itself: when one is available, an entry above *Sign out* in the side menu installs it. More details in the [Android TV README](android-tv/README.md).

## Exposing Plys to the internet

> **Warning:** Until the administrator account is created, the first visitor can claim the server and, with it, read the media folders and pair it with any server. Create the account on the local network before opening any port.

Plys serves plain http only. Over the internet, passwords and session tokens would travel in clear text, so put an HTTPS reverse proxy in front of it:

- Use [Caddy](https://caddyserver.com) or [Traefik](https://traefik.io) in front of the `web` service, with a domain name and a certificate (Let's Encrypt).
- Set `COOKIE_SECURE=true` in `.env`.
- Send `Strict-Transport-Security` (HSTS) from the proxy, so browsers never fall back to http.
- On the router, forward only ports 443 (and 80 for the certificate) to the proxy, never `8096`.
- Keep `PLYS_FEDERATION_REQUIRE_HTTPS=true`, and set the server's public URL to `https://…` in the settings before pairing.

With Caddy, add this service to `compose.yaml` (and remove the `ports` of `web` if Plys must not be reached over plain http on the local network):

```yaml
  caddy:
    image: caddy:2
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    depends_on:
      - web
```

Declare `caddy-data:` and `caddy-config:` under `volumes:` at the end of the file, and create a `Caddyfile` next to it:

```
plys.example.com {
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"
	reverse_proxy web:8080
}
```

**Login protection.** Login is limited to 20 attempts per minute per client address and 5 per minute per account, and each new block doubles the wait (up to 15 minutes). Failed attempts are logged as `login failed` with `client_ip`, which fail2ban or CrowdSec can act on. For these addresses to be right behind a proxy:

- The `web` service trusts `X-Forwarded-For` from private addresses, which covers a proxy in the same compose project or on the Docker host. For a proxy reached through a public address, or to trust only yours, mount your own `/etc/nginx/snippets/real-ip.inc` (`set_real_ip_from` lines, see `web/nginx/real-ip.inc`).
- The API trusts the compose network by default. Set `TRUSTED_PROXIES` only if the `web` service reaches the API from another network.

## Contributing

To run Plys from the sources, run the tests or publish a release, see [docs/development.md](docs/development.md).

## License

MIT, see `LICENSE`. This product uses the TMDB API but is not endorsed or certified by TMDB.
