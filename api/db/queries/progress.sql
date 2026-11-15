-- name: GetProgress :one
SELECT position_seconds, duration_seconds, watched FROM playback_progress
WHERE user_id = @user_id AND item_id = @item_id;

-- name: UpsertProgress :one
-- A watched item stays watched when played again; reaching the end resets the resume point.
INSERT INTO playback_progress (user_id, item_id, position_seconds, duration_seconds, watched, watched_at)
VALUES (@user_id, @item_id, @position_seconds, @duration_seconds, @watched, CASE WHEN @watched THEN now() END)
ON CONFLICT (user_id, item_id) DO UPDATE SET
    position_seconds = CASE WHEN playback_progress.watched OR EXCLUDED.watched THEN 0 ELSE EXCLUDED.position_seconds END,
    duration_seconds = COALESCE(EXCLUDED.duration_seconds, playback_progress.duration_seconds),
    watched          = playback_progress.watched OR EXCLUDED.watched,
    watched_at       = CASE WHEN EXCLUDED.watched AND NOT playback_progress.watched THEN now() ELSE playback_progress.watched_at END,
    updated_at       = now()
RETURNING watched;

-- name: MarkWatched :exec
INSERT INTO playback_progress (user_id, item_id, position_seconds, watched, watched_at)
SELECT @user_id, unnest(@item_ids::uuid[]), 0, true, now()
ON CONFLICT (user_id, item_id) DO UPDATE SET position_seconds = 0, watched = true, watched_at = now(), updated_at = now();

-- name: MarkUnwatched :exec
DELETE FROM playback_progress WHERE user_id = @user_id AND item_id = ANY(@item_ids::uuid[]);

-- name: ListPlayableUnder :many
-- Playable items of a node (itself, its children or grandchildren) in viewing order, with the user's progress.
SELECT i.id, i.kind, i.title, i.season_number, i.episode_number,
       COALESCE(p.watched, false) AS watched, COALESCE(p.position_seconds, 0)::float8 AS position_seconds,
       p.updated_at AS progress_at
FROM items i
LEFT JOIN playback_progress p ON p.item_id = i.id AND p.user_id = @user_id
WHERE i.kind IN ('movie', 'episode', 'video')
  AND (i.id = @root OR i.parent_id = @root OR i.parent_id IN (SELECT c.id FROM items c WHERE c.parent_id = @root))
  AND EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = i.id AND f.missing_since IS NULL AND f.probe_error IS NULL)
ORDER BY COALESCE(i.season_number, 0) = 0, i.season_number, i.episode_number, i.sort_title, i.id;

-- name: ListContinueWatching :many
SELECT i.id, i.kind, i.title, i.season_number, i.episode_number,
       COALESCE(gp.title, par.title, '')::text AS show_title, COALESCE(gp.poster_path, par.poster_path, i.poster_path) AS poster_path,
       i.thumb_path, p.position_seconds, p.duration_seconds, p.updated_at
FROM playback_progress p
JOIN items i ON i.id = p.item_id
JOIN libraries l ON l.id = i.library_id
LEFT JOIN items par ON par.id = i.parent_id
LEFT JOIN items gp ON gp.id = par.parent_id
WHERE p.user_id = @user_id AND NOT p.watched AND p.position_seconds >= @min_seconds::float8
  AND i.kind IN ('movie', 'episode', 'video')
  AND (@is_admin::bool OR NOT l.restricted
       OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
  AND EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = i.id AND f.missing_since IS NULL AND f.probe_error IS NULL)
ORDER BY p.updated_at DESC
LIMIT @lim;

-- name: ListNextUp :many
-- For each series with a watched episode: the first episode after the furthest watched one.
-- It is left out when already started, since "continue watching" shows it.
WITH eps AS (
    SELECT e.id, s.id AS series_id, sn.season_number AS sno, COALESCE(e.episode_number, 0) AS eno,
           COALESCE(p.watched, false) AS watched, COALESCE(p.position_seconds, 0) AS pos, p.updated_at AS progress_at
    FROM items e
    JOIN items sn ON sn.id = e.parent_id
    JOIN items s ON s.id = sn.parent_id
    JOIN libraries l ON l.id = s.library_id
    LEFT JOIN playback_progress p ON p.item_id = e.id AND p.user_id = @user_id
    WHERE e.kind = 'episode' AND sn.season_number > 0
      AND (@is_admin::bool OR NOT l.restricted
           OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
      AND EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = e.id AND f.missing_since IS NULL AND f.probe_error IS NULL)
),
seen AS (
    SELECT series_id, max(sno * 100000 + eno) AS furthest, max(progress_at) AS last_at
    FROM eps WHERE watched GROUP BY series_id
),
upcoming AS (
    SELECT DISTINCT ON (e.series_id) e.id, e.series_id, e.pos, seen.last_at
    FROM eps e JOIN seen USING (series_id)
    WHERE NOT e.watched AND e.sno * 100000 + e.eno > seen.furthest
    ORDER BY e.series_id, e.sno, e.eno
)
SELECT i.id, i.kind, i.title, i.season_number, i.episode_number,
       s.title AS show_title, s.poster_path AS poster_path, i.thumb_path,
       0::float8 AS position_seconds, 0::float8 AS duration_seconds, n.last_at::timestamptz AS updated_at
FROM upcoming n
JOIN items i ON i.id = n.id
JOIN items s ON s.id = n.series_id
WHERE n.pos < @min_seconds::float8
ORDER BY n.last_at DESC
LIMIT @lim;
