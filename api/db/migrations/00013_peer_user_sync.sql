-- +goose Up
-- Last attempt to re-read a peer's users; throttles the sync triggered by listing users.
ALTER TABLE peers ADD COLUMN users_sync_attempted_at timestamptz;

-- +goose Down
ALTER TABLE peers DROP COLUMN users_sync_attempted_at;
