-- +goose Up
-- Movie parts are no longer grouped. Their files lose their item (media_files.item_id is ON DELETE SET NULL),
-- so the next scan matches each of them again by its own name, and movies left empty are purged by that scan.
DELETE FROM items WHERE kind = 'part';
ALTER TABLE items DROP COLUMN part_number;
ALTER TABLE items DROP CONSTRAINT items_kind_check;
ALTER TABLE items ADD CONSTRAINT items_kind_check
    CHECK (kind IN ('movie', 'series', 'season', 'episode', 'video'));
ALTER TABLE media_files
    DROP COLUMN part_number,
    DROP COLUMN version_key;

-- +goose Down
ALTER TABLE media_files
    ADD COLUMN version_key text NOT NULL DEFAULT '',
    ADD COLUMN part_number integer;
UPDATE media_files SET version_key = lower(path);
ALTER TABLE items DROP CONSTRAINT items_kind_check;
ALTER TABLE items ADD CONSTRAINT items_kind_check
    CHECK (kind IN ('movie', 'part', 'series', 'season', 'episode', 'video'));
ALTER TABLE items ADD COLUMN part_number integer;
