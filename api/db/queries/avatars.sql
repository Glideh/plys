-- name: UpsertUserAvatar :exec
INSERT INTO user_avatars (user_id, data)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data;

-- name: GetUserAvatarData :one
SELECT data FROM user_avatars WHERE user_id = $1;

-- name: DeleteUserAvatar :exec
DELETE FROM user_avatars WHERE user_id = $1;

-- name: SetUserAvatarUpdatedAt :one
UPDATE users
SET avatar_updated_at = sqlc.narg('avatar_updated_at')
WHERE id = @id
RETURNING *;
