-- name: GetSetting :one
SELECT value FROM settings WHERE key = $1;

-- name: InsertSettingIfAbsent :exec
INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING;
