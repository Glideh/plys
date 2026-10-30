-- +goose Up
ALTER TABLE items DROP CONSTRAINT items_kind_check;
ALTER TABLE items ADD CONSTRAINT items_kind_check
    CHECK (kind IN ('movie', 'part', 'series', 'season', 'episode', 'video'));
ALTER TABLE items ADD COLUMN part_number integer;

-- +goose Down
DELETE FROM items WHERE kind = 'part';
ALTER TABLE items DROP COLUMN part_number;
ALTER TABLE items DROP CONSTRAINT items_kind_check;
ALTER TABLE items ADD CONSTRAINT items_kind_check
    CHECK (kind IN ('movie', 'series', 'season', 'episode', 'video'));
