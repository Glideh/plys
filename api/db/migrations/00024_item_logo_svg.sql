-- +goose Up
-- SVG logos are now kept (as the PNG TMDB renders from them). The enrichment pass moves to version 4: titles that
-- already have a logo are marked done, so only the ones without a logo are refreshed from TMDB again.
UPDATE items SET enrichment_version = 4 WHERE enrichment_version = 3 AND logo_path IS NOT NULL;

-- +goose Down
UPDATE items SET enrichment_version = 3 WHERE enrichment_version = 4;
