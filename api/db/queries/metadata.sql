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
-- Fields listed in locked_fields were edited by an administrator and keep their value.
UPDATE items SET
    tmdb_id             = @tmdb_id,
    imdb_id             = @imdb_id,
    title               = CASE WHEN 'title' = ANY(locked_fields) THEN title ELSE coalesce(nullif(@title::text, ''), title) END,
    sort_title          = CASE WHEN 'title' = ANY(locked_fields) THEN sort_title ELSE coalesce(nullif(@sort_title::text, ''), sort_title) END,
    original_title      = CASE WHEN 'originalTitle' = ANY(locked_fields) THEN original_title ELSE @original_title END,
    year                = CASE WHEN 'releaseDate' = ANY(locked_fields) THEN year ELSE coalesce(@year, year) END,
    overview            = CASE WHEN 'overview' = ANY(locked_fields) THEN overview ELSE @overview END,
    tagline             = CASE WHEN 'tagline' = ANY(locked_fields) THEN tagline ELSE @tagline END,
    release_date        = CASE WHEN 'releaseDate' = ANY(locked_fields) THEN release_date ELSE @release_date END,
    runtime_minutes     = @runtime_minutes,
    rating              = @rating,
    genres              = coalesce(@genres::text[], '{}'),
    cast_members        = CASE WHEN 'cast' = ANY(locked_fields) THEN cast_members ELSE coalesce(@cast_members::jsonb, '[]') END,
    directors           = CASE WHEN 'directors' = ANY(locked_fields) THEN directors ELSE coalesce(@directors::text[], '{}') END,
    director_members    = CASE WHEN 'directors' = ANY(locked_fields) THEN director_members ELSE coalesce(@director_members::jsonb, '[]') END,
    poster_path         = CASE WHEN 'poster' = ANY(locked_fields) THEN poster_path ELSE @poster_path END,
    backdrop_path       = @backdrop_path,
    logo_path           = @logo_path,
    thumb_path          = coalesce(@thumb_path, thumb_path),
    match_status        = @match_status,
    metadata_language   = @metadata_language,
    metadata_updated_at = now(),
    updated_at          = now()
WHERE id = @id;

-- name: EditItemMetadata :exec
-- An administrator's edit: the handler sends every field, merged with the current values, and the fields to lock.
UPDATE items SET
    title               = @title,
    sort_title          = @sort_title,
    original_title      = @original_title,
    year                = @year,
    release_date        = @release_date,
    tagline             = @tagline,
    overview            = @overview,
    directors           = @directors::text[],
    director_members    = @director_members::jsonb,
    cast_members        = @cast_members::jsonb,
    poster_path         = @poster_path,
    locked_fields       = @locked_fields::text[],
    metadata_updated_at = now(),
    updated_at          = now()
WHERE id = @id;

-- name: UnlockItemFields :exec
UPDATE items SET locked_fields = '{}', updated_at = now() WHERE id = $1;

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
-- Children with no metadata of their own follow their parent.
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
-- Files of the item and of its direct and nested children (seasons, episodes).
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
