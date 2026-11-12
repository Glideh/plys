-- name: ListExternalSubtitles :many
SELECT id, file_id, provider, language, release_name, hearing_impaired
FROM external_subtitles
WHERE file_id = ANY(@file_ids::uuid[])
ORDER BY language, created_at;

-- name: GetExternalSubtitle :one
SELECT id, file_id, provider, language, release_name, hearing_impaired
FROM external_subtitles
WHERE id = $1;

-- name: GetExternalSubtitleByProvider :one
SELECT id, file_id, provider, language, release_name, hearing_impaired
FROM external_subtitles
WHERE file_id = $1 AND provider = $2 AND provider_file_id = $3;

-- name: GetExternalSubtitleVTT :one
SELECT vtt FROM external_subtitles WHERE id = $1;

-- name: InsertExternalSubtitle :one
-- A concurrent download of the same file keeps the first copy.
INSERT INTO external_subtitles (file_id, provider, provider_file_id, language, release_name, hearing_impaired, vtt, created_by)
VALUES (@file_id, @provider, @provider_file_id, @language, @release_name, @hearing_impaired, @vtt, @created_by)
ON CONFLICT (file_id, provider, provider_file_id) DO UPDATE SET provider = EXCLUDED.provider
RETURNING id, file_id, provider, language, release_name, hearing_impaired;

-- name: DeleteExternalSubtitle :execrows
DELETE FROM external_subtitles WHERE id = $1 AND file_id = $2;
