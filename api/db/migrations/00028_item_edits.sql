-- +goose Up
-- Fields an administrator edited by hand ("title", "originalTitle", "releaseDate", "tagline", "overview",
-- "directors", "cast", "poster"). Metadata refreshes leave them as they are until the edits are reverted.
ALTER TABLE items ADD COLUMN locked_fields text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE items DROP COLUMN locked_fields;
