-- +goose Up
-- TMDB title logo ("/abc.png"), shown by the players while paused. Filled by the enrichment pass
-- (enrichment_version 3), which refreshes already matched titles once.
ALTER TABLE items ADD COLUMN logo_path text;

-- +goose Down
ALTER TABLE items DROP COLUMN logo_path;
