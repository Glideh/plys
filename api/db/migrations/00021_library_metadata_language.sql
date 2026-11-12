-- +goose Up
-- Optional TMDB language for one library. NULL follows the server-wide metadata language.
ALTER TABLE libraries ADD COLUMN metadata_language text;

-- +goose Down
ALTER TABLE libraries DROP COLUMN metadata_language;
