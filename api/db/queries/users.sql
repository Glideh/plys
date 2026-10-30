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
SELECT * FROM users WHERE lower(username) = lower(@username);

-- name: ListUsers :many
SELECT * FROM users ORDER BY lower(username);

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
