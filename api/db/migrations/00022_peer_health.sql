-- +goose Up
-- Reachability of each peer, so that fan-outs skip an offline peer instead of waiting for its timeout.
ALTER TABLE peers ADD COLUMN paused boolean NOT NULL DEFAULT false;
ALTER TABLE peers ADD COLUMN consecutive_failures integer NOT NULL DEFAULT 0;
ALTER TABLE peers ADD COLUMN last_failure_at timestamptz;
ALTER TABLE peers ADD COLUMN offline_since timestamptz;
ALTER TABLE peers ADD COLUMN last_seen_at timestamptz;

-- +goose Down
ALTER TABLE peers DROP COLUMN last_seen_at;
ALTER TABLE peers DROP COLUMN offline_since;
ALTER TABLE peers DROP COLUMN last_failure_at;
ALTER TABLE peers DROP COLUMN consecutive_failures;
ALTER TABLE peers DROP COLUMN paused;
