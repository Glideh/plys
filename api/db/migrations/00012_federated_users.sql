-- +goose Up
-- A federated user is a shadow row mirroring a user of a paired peer. It never authenticates here:
-- its password_hash is empty and the username lookup used by login ignores it.
ALTER TABLE users ADD COLUMN origin_peer_id uuid REFERENCES peers (id) ON DELETE CASCADE;

DROP INDEX users_username_lower_idx;
-- Local usernames stay unique among themselves; shadow usernames only need to be unique per peer.
CREATE UNIQUE INDEX users_username_local_idx ON users (lower(username)) WHERE origin_peer_id IS NULL;
CREATE UNIQUE INDEX users_username_remote_idx ON users (origin_peer_id, lower(username)) WHERE origin_peer_id IS NOT NULL;

-- Libraries are private until an admin grants access: existing ones included.
UPDATE libraries SET restricted = true;
ALTER TABLE libraries ALTER COLUMN restricted SET DEFAULT true;

-- +goose Down
ALTER TABLE libraries ALTER COLUMN restricted SET DEFAULT false;

DELETE FROM users WHERE origin_peer_id IS NOT NULL;
DROP INDEX users_username_remote_idx;
DROP INDEX users_username_local_idx;
CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username));
ALTER TABLE users DROP COLUMN origin_peer_id;
