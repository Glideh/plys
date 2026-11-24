-- name: CreateDeviceLogin :one
INSERT INTO device_logins (device_code_hash, user_code, device_name, expires_at)
VALUES (@device_code_hash, @user_code, @device_name, @expires_at)
RETURNING *;

-- name: DeleteExpiredDeviceLogins :execrows
DELETE FROM device_logins WHERE expires_at <= now();

-- name: GetPendingDeviceLogin :one
SELECT * FROM device_logins WHERE user_code = @user_code AND user_id IS NULL AND expires_at > now();

-- name: ApproveDeviceLogin :execrows
UPDATE device_logins SET user_id = @user_id, approved_at = now()
WHERE user_code = @user_code AND user_id IS NULL AND expires_at > now();

-- name: DenyDeviceLogin :execrows
DELETE FROM device_logins WHERE user_code = @user_code AND user_id IS NULL AND expires_at > now();

-- name: GetDeviceLoginByDeviceCodeForUpdate :one
SELECT * FROM device_logins WHERE device_code_hash = @device_code_hash AND expires_at > now() FOR UPDATE;

-- name: DeleteDeviceLogin :exec
DELETE FROM device_logins WHERE id = $1;
