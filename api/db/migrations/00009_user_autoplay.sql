-- +goose Up
-- Per-user preference, kept server-side so every device (web, mobile) behaves the same.
ALTER TABLE users ADD COLUMN autoplay_next boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN autoplay_next;
