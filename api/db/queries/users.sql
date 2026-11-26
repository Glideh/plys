-- name: LockUsers :exec
-- Serializes setup and admin-count checks across concurrent requests.
SELECT pg_advisory_xact_lock(7461001);

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: CountAdmins :one
SELECT count(*) FROM users WHERE role = 'admin';

-- name: CreateUser :one
INSERT INTO users (username, password_hash, role, locale)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
-- Shadow users of paired peers never log in here.
SELECT * FROM users WHERE lower(username) = lower(@username) AND origin_peer_id IS NULL;

-- name: ListLocalUsers :many
-- Public listing served to paired peers: no credential or contact data.
SELECT id, username FROM users WHERE origin_peer_id IS NULL ORDER BY lower(username);

-- name: ListRemoteUserIDs :many
SELECT id FROM users WHERE origin_peer_id = @origin_peer_id;

-- name: UpsertRemoteUser :exec
INSERT INTO users (id, username, password_hash, role, origin_peer_id)
VALUES (@id, @username, '', 'user', @origin_peer_id)
ON CONFLICT (id) DO UPDATE SET username = EXCLUDED.username, updated_at = now()
WHERE users.origin_peer_id = EXCLUDED.origin_peer_id;

-- name: ListUsers :many
SELECT * FROM users ORDER BY lower(coalesce(alias, username));

-- name: ListPlaybackUsers :many
SELECT id, username, alias, avatar_updated_at FROM users WHERE id = ANY(@user_ids::uuid[]);

-- name: UpdateUser :one
UPDATE users
SET role          = coalesce(sqlc.narg('role'), role),
    locale        = coalesce(sqlc.narg('locale'), locale),
    password_hash = coalesce(sqlc.narg('password_hash'), password_hash),
    updated_at    = now()
WHERE id = @id
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateUserPreferences :one
UPDATE users
SET autoplay_next = coalesce(sqlc.narg('autoplay_next'), autoplay_next),
    locale        = coalesce(sqlc.narg('locale'), locale),
    updated_at    = now()
WHERE id = @id
RETURNING *;

-- name: SetUserAlias :one
UPDATE users SET alias = @alias, updated_at = now() WHERE id = @id RETURNING *;
