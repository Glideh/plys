-- +goose Up
-- One row per user and playable item (movie, episode, part, video). A missing row means never played.
-- position_seconds is the resume point and is reset to 0 once the item is watched.
CREATE TABLE playback_progress (
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id          uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    position_seconds double precision NOT NULL DEFAULT 0,
    duration_seconds double precision,
    watched          boolean NOT NULL DEFAULT false,
    watched_at       timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, item_id)
);

CREATE INDEX playback_progress_item_idx ON playback_progress (item_id);
CREATE INDEX playback_progress_recent_idx ON playback_progress (user_id, updated_at DESC);

-- +goose Down
DROP TABLE playback_progress;
