-- +goose Up
-- Directors (movies) or creators (series) with their TMDB profile picture, shaped like cast_members.
-- Filled by the enrichment pass (enrichment_version 2), which refreshes already matched titles once.
ALTER TABLE items ADD COLUMN director_members jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE items DROP COLUMN director_members;
