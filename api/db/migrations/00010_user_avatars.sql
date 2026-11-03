-- +goose Up
-- Image bytes live apart from users so that user queries (SELECT *) never load them.
CREATE TABLE user_avatars (
    user_id uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    data    bytea NOT NULL
);

ALTER TABLE users ADD COLUMN avatar_updated_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_updated_at;
DROP TABLE user_avatars;
