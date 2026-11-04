-- +goose Up
ALTER TABLE peers ADD COLUMN version text;
ALTER TABLE peers ADD COLUMN version_checked_at timestamptz;

-- +goose Down
ALTER TABLE peers DROP COLUMN version_checked_at;
ALTER TABLE peers DROP COLUMN version;