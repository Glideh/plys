-- +goose Up
ALTER TABLE items
    ADD COLUMN tmdb_id             integer,
    ADD COLUMN imdb_id             text,
    ADD COLUMN original_title      text,
    ADD COLUMN overview            text,
    ADD COLUMN tagline             text,
    ADD COLUMN release_date        date,
    ADD COLUMN runtime_minutes     integer,
    ADD COLUMN rating              real,
    ADD COLUMN genres              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN cast_members        jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN directors           text[] NOT NULL DEFAULT '{}',
    -- TMDB image paths ("/abc.jpg") or "local/<item id>.jpg" for thumbnails extracted with ffmpeg.
    ADD COLUMN poster_path         text,
    ADD COLUMN backdrop_path       text,
    ADD COLUMN thumb_path          text,
    -- manual: chosen by an admin, never overwritten by automatic matching.
    ADD COLUMN match_status        text NOT NULL DEFAULT 'pending'
        CHECK (match_status IN ('pending', 'matched', 'unmatched', 'manual', 'skipped')),
    ADD COLUMN metadata_language   text,
    ADD COLUMN metadata_updated_at timestamptz;

CREATE INDEX items_match_pending_idx ON items (library_id) WHERE match_status = 'pending';
CREATE INDEX items_tmdb_idx ON items (library_id, kind, tmdb_id) WHERE tmdb_id IS NOT NULL;

-- +goose Down
DROP INDEX items_tmdb_idx;
DROP INDEX items_match_pending_idx;
ALTER TABLE items
    DROP COLUMN tmdb_id, DROP COLUMN imdb_id, DROP COLUMN original_title, DROP COLUMN overview,
    DROP COLUMN tagline, DROP COLUMN release_date, DROP COLUMN runtime_minutes, DROP COLUMN rating,
    DROP COLUMN genres, DROP COLUMN cast_members, DROP COLUMN directors, DROP COLUMN poster_path,
    DROP COLUMN backdrop_path, DROP COLUMN thumb_path, DROP COLUMN match_status,
    DROP COLUMN metadata_language, DROP COLUMN metadata_updated_at;
