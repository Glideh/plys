-- +goose Up
-- Aliases are local labels set by an administrator; clients show them instead of the original name.
ALTER TABLE users ADD COLUMN alias text;
ALTER TABLE peers ADD COLUMN alias text;

-- +goose Down
ALTER TABLE peers DROP COLUMN alias;
ALTER TABLE users DROP COLUMN alias;
