-- +goose Up
-- IMDb public rating and vote count, read from the daily IMDb dataset by the item's imdb_id.
ALTER TABLE items
    ADD COLUMN imdb_rating real,
    ADD COLUMN imdb_votes  integer;

-- +goose Down
ALTER TABLE items DROP COLUMN imdb_rating, DROP COLUMN imdb_votes;
