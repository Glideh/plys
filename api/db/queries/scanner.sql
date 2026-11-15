-- name: ListLibraryFiles :many
SELECT id, path, size, mtime, item_id, probed_at, missing_since FROM media_files WHERE library_id = $1;

-- name: UpsertItem :one
-- Existing titles are kept: metadata providers own them once the item is matched.
INSERT INTO items (library_id, parent_id, kind, match_key, title, sort_title, year, season_number, episode_number, episode_number_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT ON CONSTRAINT items_identity DO UPDATE SET updated_at = now()
RETURNING id, (xmax = 0) AS inserted;

-- name: UpsertProbedFile :one
INSERT INTO media_files (library_id, item_id, path, size, mtime, duration_seconds, container, bitrate, streams, probe_error, edition, probed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
ON CONFLICT (path) DO UPDATE SET
    library_id       = EXCLUDED.library_id,
    item_id          = EXCLUDED.item_id,
    size             = EXCLUDED.size,
    mtime            = EXCLUDED.mtime,
    duration_seconds = EXCLUDED.duration_seconds,
    container        = EXCLUDED.container,
    bitrate          = EXCLUDED.bitrate,
    streams          = EXCLUDED.streams,
    probe_error      = EXCLUDED.probe_error,
    edition          = EXCLUDED.edition,
    probed_at        = EXCLUDED.probed_at,
    missing_since    = NULL,
    updated_at       = now()
RETURNING id;

-- name: LinkFileItem :exec
UPDATE media_files
SET item_id = $2, edition = $3, updated_at = now()
WHERE id = $1;

-- name: ClearFilesMissing :exec
UPDATE media_files SET missing_since = NULL WHERE id = ANY(@ids::uuid[]) AND missing_since IS NOT NULL;

-- name: MarkFilesMissing :execrows
UPDATE media_files SET missing_since = now()
WHERE library_id = @library_id AND missing_since IS NULL AND NOT (id = ANY(@seen::uuid[]));

-- name: PurgeMissingFiles :execrows
DELETE FROM media_files WHERE library_id = $1 AND missing_since < $2;

-- name: DeleteOrphanLeaves :execrows
DELETE FROM items i
WHERE i.library_id = $1 AND i.kind IN ('movie', 'episode', 'video')
  AND NOT EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = i.id);

-- name: DeleteEmptyContainers :execrows
-- Run twice: empty seasons first, then series left without seasons.
DELETE FROM items i
WHERE i.library_id = $1 AND i.kind IN ('season', 'series')
  AND NOT EXISTS (SELECT 1 FROM items c WHERE c.parent_id = i.id);

-- name: SetLibraryScanned :exec
UPDATE libraries SET last_scanned_at = now() WHERE id = $1;

-- name: CountItemsByLibrary :many
SELECT library_id, kind, count(*) FROM items GROUP BY library_id, kind;
