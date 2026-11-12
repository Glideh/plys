-- +goose Up
-- Titles a user plans to watch: top-level items only (movies, series, videos).
-- A title held by a paired server is listed there, under the user's shadow account.
CREATE TABLE watchlist (
    user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id  uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    added_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, item_id)
);

CREATE INDEX watchlist_item_idx ON watchlist (item_id);
CREATE INDEX watchlist_recent_idx ON watchlist (user_id, added_at DESC);

-- +goose Down
DROP TABLE watchlist;
