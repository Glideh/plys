-- +goose Up
-- Files of one item sharing a version_key are parts played in sequence; distinct keys are alternative versions.
ALTER TABLE media_files
    ADD COLUMN version_key text NOT NULL DEFAULT '',
    ADD COLUMN part_number integer,
    ADD COLUMN edition     text;

UPDATE media_files SET version_key = lower(path);

-- +goose Down
ALTER TABLE media_files
    DROP COLUMN edition,
    DROP COLUMN part_number,
    DROP COLUMN version_key;
