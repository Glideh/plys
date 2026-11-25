# Database Schema

Plys stores its application data in PostgreSQL. The schema is defined by the Goose migrations in [`api/db/migrations`](../api/db/migrations) and queried through sqlc.

## Entity Relationship Diagram

```mermaid
erDiagram
    USERS ||--o{ REFRESH_TOKENS : owns
    PEERS o|--o{ USERS : mirrors
    USERS ||--o| USER_AVATARS : has
    USERS ||--o{ LIBRARY_ACCESS : receives
    LIBRARIES ||--o{ LIBRARY_ACCESS : grants
    USERS ||--o{ PLAYBACK_PROGRESS : records
    ITEMS ||--o{ PLAYBACK_PROGRESS : tracks
    USERS ||--o{ WATCHLIST : saves
    ITEMS ||--o{ WATCHLIST : contains
    USERS o|--o{ EXTERNAL_SUBTITLES : downloads
    MEDIA_FILES ||--o{ EXTERNAL_SUBTITLES : has
    USERS o|--o{ INVITATIONS : creates
    USERS o|--o{ DEVICE_LOGINS : approves
    LIBRARIES ||--o{ LIBRARY_PATHS : includes
    LIBRARIES ||--o{ ITEMS : owns
    ITEMS o|--o{ ITEMS : parents
    LIBRARIES ||--o{ MEDIA_FILES : owns
    ITEMS o|--o{ MEDIA_FILES : identifies

    USERS {
        uuid id PK
        text username
        text password_hash
        text role
        text locale
        boolean autoplay_next
        uuid origin_peer_id FK
        text alias
    }
    REFRESH_TOKENS {
        uuid id PK
        uuid user_id FK
        uuid family_id
        bytea token_hash UK
        timestamp expires_at
        timestamp revoked_at
    }
    SETTINGS {
        text key PK
        jsonb value
        timestamp updated_at
    }
    LIBRARIES {
        uuid id PK
        text name UK
        text kind
        boolean restricted
        text metadata_language
        timestamp last_scanned_at
    }
    LIBRARY_PATHS {
        uuid library_id PK,FK
        text path PK,UK
    }
    ITEMS {
        uuid id PK
        uuid library_id FK
        uuid parent_id FK
        text kind
        text match_key
        text title
        integer tmdb_id
        integer fankai_id
        text imdb_id
        real rating
        real imdb_rating
        integer imdb_votes
        uuid collection_id
        text match_status
        integer enrichment_version
    }
    MEDIA_FILES {
        uuid id PK
        uuid library_id FK
        uuid item_id FK
        text path UK
        bigint size
        timestamp mtime
        jsonb streams
        text edition
        timestamp missing_since
    }
    LIBRARY_ACCESS {
        uuid library_id PK,FK
        uuid user_id PK,FK
    }
    PLAYBACK_PROGRESS {
        uuid user_id PK,FK
        uuid item_id PK,FK
        float position_seconds
        float duration_seconds
        boolean watched
        timestamp updated_at
    }
    USER_AVATARS {
        uuid user_id PK,FK
        bytea data
    }
    PEERS {
        uuid id PK
        text url UK
        text name
        bytea public_key
        boolean paused
        text alias
    }
    WATCHLIST {
        uuid user_id PK,FK
        uuid item_id PK,FK
        timestamp added_at
    }
    EXTERNAL_SUBTITLES {
        uuid id PK
        uuid file_id FK
        text provider
        text provider_file_id
        text language
        text vtt
        uuid created_by FK
    }
    INVITATIONS {
        uuid id PK
        bytea token_hash UK
        bytea pin_hash
        text locale
        uuid created_by FK
        integer failed_attempts
        timestamp expires_at
    }
    DEVICE_LOGINS {
        uuid id PK
        bytea device_code_hash UK
        text user_code UK
        text device_name
        uuid user_id FK
        timestamp approved_at
        timestamp created_at
        timestamp expires_at
    }
```

`PK` means primary key, `FK` means foreign key, and `UK` means a unique key or index.

## Tables

### `users`

Stores local accounts and shadow accounts mirrored from paired servers. A local username is unique among local users, while a federated username is unique within its origin peer. The `role` controls administration rights, `autoplay_next` stores the playback preference, and `alias` changes the displayed name. Deleting a peer cascades to all of its shadow users.

### `refresh_tokens`

Stores hashed refresh-token families for persistent authentication. Each token belongs to one user and has a device name, expiry time, and optional revocation time. Tokens are rotated in families so reuse of an older token can invalidate related sessions.

### `settings`

A small key-value store for server-wide configuration. Values are kept as JSON so settings can evolve independently of schema migrations. Examples include API credentials and metadata preferences.

### `libraries`

Defines a media library, its type (`movies`, `series`, or `other`), and its scan metadata. Libraries are restricted by default; administrators see every library, while regular users require a matching `library_access` row. `metadata_language` overrides the server-wide metadata language.

### `library_paths`

Maps a library to one or more resolved filesystem paths. A path can belong to only one library, and deleting a library deletes all of its path mappings.

### `items`

The catalogue tree. A single table represents movies, series, seasons, episodes, and personal videos. `parent_id` forms the series-to-season-to-episode hierarchy. The `library_id`, sibling identity, kind, and `match_key` prevent duplicate nodes during rescans. `tmdb_id` and `fankai_id` are separate provider identities; FanKai IDs are never used as TMDB IDs for title grouping, watchlists, or subtitles.

Items with the same `collection_id` belong to the same collection; the identifier does not reference a separate table.

This table also stores enriched metadata from TMDB, including identifiers, titles, overview, release data, runtime, rating, cast, crew, artwork paths, genres, and filter dimensions. `locked_fields` preserves administrator edits when metadata is refreshed. `enrichment_version` records the latest completed metadata migration for the item. `imdb_rating` and `imdb_votes` hold the IMDb public rating, refreshed daily from IMDb's free `title.ratings.tsv.gz` dataset through `imdb_id`.

### `media_files`

Stores discovered playable files and their probe results. Each file has an immutable path identity plus size, modification time, technical streams, duration, container, bitrate, and optional edition. `item_id` can be null while a file is unidentified; deleting an item leaves the file available for rescanning by setting `item_id` to null.

`missing_since` supports a grace period before permanently removing a file that is no longer visible, which avoids wiping catalogue data during a temporary mount failure.

### `library_access`

A join table granting a regular user access to a restricted library. The composite primary key prevents duplicate grants. Both sides cascade on deletion.

### `playback_progress`

Stores one resume/watch state per user and catalogue item. It contains the resume position, known duration, watched state, watch timestamp, and last update. The table drives resume rails, next-episode choices, and watched filters.

### `user_avatars`

Stores an uploaded avatar as binary data. There is at most one row per user, and the row is deleted with its user. `users.avatar_updated_at` is used for cache invalidation.

### `peers`

Stores another Plys server that has been paired with this instance. It contains the remote server identity, URL, public key, display alias, compatibility/version state, pause state, and connectivity health. A peer can own federated shadow users through `users.origin_peer_id`.

### `watchlist`

Stores the top-level titles a user wants to watch. The same user/item pair can appear only once. Deleting a user or catalogue item removes the entry automatically.

### `external_subtitles`

Stores downloaded online subtitles for a media file. The WebVTT payload is kept in PostgreSQL so every authorized user can access it even when the external provider or download cache is unavailable. Subtitle uniqueness is enforced per file, provider, and provider file ID. Deleting the creator preserves the subtitle and sets `created_by` to null.

### `invitations`

Stores short-lived account invitations. Only hashes of the invitation token and PIN are persisted. Failed attempts are tracked, and `expires_at` controls validity. Deleting the administrator who created an invitation keeps the invitation but sets `created_by` to null.

### `device_logins`

Stores temporary sign-in requests initiated by a TV and approved by a user on another device. Only the hash of the TV's secret device code is stored. The user and approval time are optional until approval; the request expires or is deleted when the TV retrieves its session. Deleting the approving user deletes the request.

## Supporting Database Features

- Most UUID primary keys default to `gen_random_uuid()`; `peers.id` is supplied by the paired server.
- Destructive catalogue relationships generally use `ON DELETE CASCADE` so a deleted library or parent item cannot leave orphan records.
- `search_fold(text)` provides accent-insensitive text search through PostgreSQL `unaccent`.
- `media_resolution(jsonb)` and `media_audio_formats(jsonb)` derive technical filters from probed stream metadata.
- GIN indexes on genre, actor, creator, and keyword ID arrays accelerate advanced catalogue filters.
- Partial indexes speed pending metadata enrichment and TMDB lookup.

The SQL migrations remain the source of truth for exact column defaults, checks, and index definitions.
