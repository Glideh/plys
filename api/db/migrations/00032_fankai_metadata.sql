-- +goose Up
-- A FanKai match is a different identity from a TMDB match.
ALTER TABLE items ADD COLUMN fankai_id integer;

-- +goose Down
ALTER TABLE items DROP COLUMN fankai_id;
