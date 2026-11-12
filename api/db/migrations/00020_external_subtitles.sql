-- +goose Up
-- Subtitles downloaded from an online provider for one file, shared by every user who can play it.
-- The WebVTT text is kept in the database: the media folders are read-only and the cache may be wiped.
CREATE TABLE external_subtitles (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id          uuid NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    provider         text NOT NULL,
    provider_file_id text NOT NULL,
    language         text NOT NULL,
    release_name     text NOT NULL DEFAULT '',
    hearing_impaired boolean NOT NULL DEFAULT false,
    vtt              text NOT NULL,
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (file_id, provider, provider_file_id)
);

-- +goose Down
DROP TABLE external_subtitles;
