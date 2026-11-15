-- name: ListItemsToMatch :many
-- Series are revisited while any of their seasons or episodes is still pending (new episodes).
SELECT id, kind, title, year, tmdb_id, match_status FROM items i
WHERE i.library_id = @library_id AND i.kind IN ('movie', 'series', 'video')
  AND (
    i.match_status = 'pending'
    OR (i.kind IN ('movie', 'series') AND i.match_status IN ('matched', 'manual') AND i.enrichment_version < 4)
    OR (i.match_status IN ('matched', 'manual') AND i.metadata_language IS DISTINCT FROM @language::text)
    OR (i.kind = 'series' AND EXISTS (
        SELECT 1 FROM items s
        WHERE s.parent_id = i.id
          AND (s.match_status = 'pending' OR EXISTS (SELECT 1 FROM items e WHERE e.parent_id = s.id AND e.match_status = 'pending'))
    ))
  )
ORDER BY i.created_at;

-- name: GetItem :one
SELECT * FROM items WHERE id = $1;

-- name: ListLibrariesForFilterEnrichment :many
SELECT l.* FROM libraries l WHERE EXISTS (
    SELECT 1 FROM items i WHERE i.library_id = l.id AND i.kind IN ('movie', 'series')
      AND i.match_status IN ('matched', 'manual') AND i.enrichment_version < 4
);

-- name: UpdateItemMetadata :exec
UPDATE items SET
    tmdb_id             = @tmdb_id,
    imdb_id             = @imdb_id,
    title               = coalesce(nullif(@title::text, ''), title),
    sort_title          = coalesce(nullif(@sort_title::text, ''), sort_title),
    original_title      = @original_title,
    year                = coalesce(@year, year),
    overview            = @overview,
    tagline             = @tagline,
    release_date        = @release_date,
    runtime_minutes     = @runtime_minutes,
    rating              = @rating,
    genres              = coalesce(@genres::text[], '{}'),
    cast_members        = coalesce(@cast_members::jsonb, '[]'),
    directors           = coalesce(@directors::text[], '{}'),
    director_members    = coalesce(@director_members::jsonb, '[]'),
    poster_path         = @poster_path,
    backdrop_path       = @backdrop_path,
    logo_path           = @logo_path,
    thumb_path          = coalesce(@thumb_path, thumb_path),
    match_status        = @match_status,
    metadata_language   = @metadata_language,
    metadata_updated_at = now(),
    updated_at          = now()
WHERE id = @id;

-- name: SetItemMatchStatus :exec
UPDATE items SET match_status = $2, metadata_updated_at = now() WHERE id = $1;

-- name: UpdateItemFilters :exec
UPDATE items SET genre_ids = @genre_ids::integer[], actor_ids = @actor_ids::integer[],
  creator_ids = @creator_ids::integer[], keyword_ids = @keyword_ids::integer[],
  original_language = @original_language, countries = @countries::text[], vote_count = @vote_count,
  filter_labels = @filter_labels::jsonb, enrichment_version = 4
WHERE id = @id;

-- name: SetItemThumb :exec
UPDATE items SET thumb_path = $2, match_status = $3, metadata_updated_at = now() WHERE id = $1;

-- name: SetChildrenMatchStatus :exec
-- Children with no metadata of their own (movie parts) follow their parent.
UPDATE items SET match_status = $2 WHERE parent_id = $1 AND match_status = 'pending';

-- name: ListChildren :many
SELECT id, kind, match_key, season_number, episode_number, match_status FROM items WHERE parent_id = $1 ORDER BY sort_title;

-- name: FindChild :one
SELECT id FROM items WHERE parent_id = @parent_id AND kind = @kind AND match_key = @match_key;

-- name: MoveItem :exec
UPDATE items SET parent_id = $2, updated_at = now() WHERE id = $1;

-- name: MoveItemFiles :exec
UPDATE media_files SET item_id = @to_item, updated_at = now() WHERE item_id = @from_item;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;

-- name: FindOtherItemByTmdb :one
-- The oldest item wins a merge so ids already shared with clients stay stable.
SELECT id FROM items
WHERE library_id = @library_id AND kind = @kind AND tmdb_id = @tmdb_id AND id <> @exclude AND parent_id IS NULL
ORDER BY created_at
LIMIT 1;

-- name: ListItemFilePaths :many
-- Files of the item and of its direct and nested children (parts, seasons, episodes).
SELECT f.path FROM media_files f
WHERE f.missing_since IS NULL AND f.item_id IN (
    SELECT @item_id::uuid
    UNION SELECT c.id FROM items c WHERE c.parent_id = @item_id
    UNION SELECT g.id FROM items c JOIN items g ON g.parent_id = c.id WHERE c.parent_id = @item_id
)
ORDER BY f.path
LIMIT @max_paths;

-- name: GetItemThumbSource :one
SELECT path, duration_seconds FROM media_files
WHERE item_id = $1 AND missing_since IS NULL AND probe_error IS NULL
ORDER BY size DESC
LIMIT 1;

-- name: IsImagePathReferenced :one
SELECT EXISTS (
    SELECT 1 FROM items
    WHERE poster_path = @path::text OR backdrop_path = @path::text OR thumb_path = @path::text OR logo_path = @path::text
       OR cast_members @> jsonb_build_array(jsonb_build_object('profilePath', @path::text))
       OR director_members @> jsonb_build_array(jsonb_build_object('profilePath', @path::text))
);

-- name: ListTMDBPosterPaths :many
SELECT DISTINCT poster_path::text FROM items
WHERE poster_path LIKE '/%' AND kind IN ('movie', 'series', 'season');

-- name: CountMatchStatusByLibrary :many
SELECT library_id, match_status, count(*) FROM items WHERE kind IN ('movie', 'series', 'video') GROUP BY library_id, match_status;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = $1;
