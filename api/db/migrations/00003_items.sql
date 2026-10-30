-- +goose Up
ALTER TABLE libraries ADD COLUMN last_scanned_at timestamptz;

-- One table for every catalogue node keeps "continue watching" and "next up" queries simple.
-- match_key identifies a node among its siblings so rescans update instead of duplicating.
CREATE TABLE items (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id         uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    parent_id          uuid REFERENCES items (id) ON DELETE CASCADE,
    kind               text NOT NULL CHECK (kind IN ('movie', 'series', 'season', 'episode', 'video')),
    match_key          text NOT NULL,
    title              text NOT NULL,
    sort_title         text NOT NULL,
    year               integer,
    season_number      integer,
    episode_number     integer,
    episode_number_end integer,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT items_identity UNIQUE NULLS NOT DISTINCT (library_id, parent_id, kind, match_key)
);

CREATE INDEX items_parent_idx ON items (parent_id);
CREATE INDEX items_library_kind_idx ON items (library_id, kind);

CREATE TABLE media_files (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id       uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    item_id          uuid REFERENCES items (id) ON DELETE SET NULL,
    path             text NOT NULL UNIQUE,
    size             bigint NOT NULL,
    mtime            timestamptz NOT NULL,
    duration_seconds double precision,
    container        text,
    bitrate          bigint,
    streams          jsonb NOT NULL DEFAULT '[]',
    probe_error      text,
    probed_at        timestamptz,
    -- Set when a scan no longer finds the file; purged after a grace period so an unmounted share does not wipe the catalogue.
    missing_since    timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX media_files_library_idx ON media_files (library_id);
CREATE INDEX media_files_item_idx ON media_files (item_id);

-- +goose Down
DROP TABLE media_files;
DROP TABLE items;
ALTER TABLE libraries DROP COLUMN last_scanned_at;
