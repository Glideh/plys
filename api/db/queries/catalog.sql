-- name: CanAccessLibrary :one
SELECT EXISTS (
    SELECT 1 FROM libraries l
    WHERE l.id = @library_id
      AND (@is_admin::bool OR NOT l.restricted
           OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
);

-- name: ListLibraryAccess :many
SELECT library_id, user_id FROM library_access;

-- name: SetLibraryRestricted :exec
UPDATE libraries SET restricted = $2, updated_at = now() WHERE id = $1;

-- name: ClearLibraryAccess :exec
DELETE FROM library_access WHERE library_id = $1;

-- name: AddLibraryAccess :exec
INSERT INTO library_access (library_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListLibraryItems :many
-- Top-level entries of a library; entries whose files are all missing are hidden.
-- Each file is mapped once to its top-level ancestor: a per-entry subquery over all files was quadratic.
WITH visible AS (
    SELECT DISTINCT COALESCE(p.parent_id, x.parent_id, x.id) AS root_id
    FROM media_files f
    JOIN items x ON x.id = f.item_id
    LEFT JOIN items p ON p.id = x.parent_id
    WHERE f.missing_since IS NULL AND x.library_id = @library_id
),
-- Episodes still to watch per series, for the badge of the grid.
unwatched AS (
    SELECT sn.parent_id AS series_id, count(DISTINCT x.id)::int AS n
    FROM media_files f
    JOIN items x ON x.id = f.item_id AND x.kind = 'episode'
    JOIN items sn ON sn.id = x.parent_id
    LEFT JOIN playback_progress pr ON pr.item_id = x.id AND pr.user_id = @user_id
    WHERE f.missing_since IS NULL AND x.library_id = @library_id AND pr.watched IS NOT TRUE
    GROUP BY sn.parent_id
)
SELECT i.id, i.kind, i.title, i.year, i.poster_path, i.thumb_path, i.rating, i.match_status,
       (SELECT count(*) FROM items e JOIN items s ON e.parent_id = s.id WHERE s.parent_id = i.id)::int AS episode_count,
       COALESCE(u.n, 0)::int AS unwatched_count,
       COALESCE(pw.watched, false) AS watched,
       count(*) OVER ()::int AS total
FROM items i
LEFT JOIN unwatched u ON u.series_id = i.id
LEFT JOIN playback_progress pw ON pw.item_id = i.id AND pw.user_id = @user_id
WHERE i.library_id = @library_id AND i.parent_id IS NULL
  AND (@query::text = '' OR i.title ILIKE @query OR i.original_title ILIKE @query)
  AND i.id IN (SELECT root_id FROM visible)
ORDER BY
    CASE WHEN @sort::text = 'year' THEN i.year END DESC NULLS LAST,
    CASE WHEN @sort::text = 'added' THEN i.created_at END DESC,
    CASE WHEN @sort::text = 'rating' THEN i.rating END DESC NULLS LAST,
    i.sort_title, i.id
LIMIT @lim OFFSET @off;

-- name: ListChildItems :many
SELECT i.id, i.kind, i.title, i.overview, i.year, i.release_date, i.runtime_minutes, i.rating,
       i.poster_path, i.thumb_path, i.season_number, i.episode_number, i.episode_number_end, i.part_number,
       (SELECT count(*) FROM items c WHERE c.parent_id = i.id)::int AS child_count,
       COALESCE(p.watched, false) AS watched,
       COALESCE(p.position_seconds, 0)::float8 AS position_seconds,
       (SELECT count(*) FROM items e
         WHERE e.parent_id = i.id AND e.kind = 'episode'
           AND EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = e.id AND f.missing_since IS NULL)
           AND NOT EXISTS (SELECT 1 FROM playback_progress q WHERE q.item_id = e.id AND q.user_id = @user_id AND q.watched))::int AS unwatched_count
FROM items i
LEFT JOIN playback_progress p ON p.item_id = i.id AND p.user_id = @user_id
WHERE i.parent_id = @parent_id
ORDER BY i.sort_title, i.id;

-- name: GetPlaybackFile :one
SELECT f.id, f.path, f.size, f.duration_seconds, f.streams, f.item_id, f.library_id
FROM media_files f
WHERE f.id = $1 AND f.missing_since IS NULL AND f.probe_error IS NULL;

-- name: ListItemFiles :many
-- Paths stay server-side: clients only see technical details.
SELECT id, item_id, size, duration_seconds, container, streams, edition, probe_error
FROM media_files
WHERE item_id = ANY(@item_ids::uuid[]) AND missing_since IS NULL
ORDER BY size DESC;
