-- name: ListCollectionCards :many
-- The other titles of the item's collection that the user can read, in release order; titles whose files are all
-- missing are hidden.
SELECT i.id, i.kind, i.title, i.year, i.poster_path, i.thumb_path, i.rating, i.match_status,
       COALESCE(p.watched, false) AS watched
FROM items me
JOIN items i ON i.collection_id = me.collection_id AND i.id <> me.id
JOIN libraries l ON l.id = i.library_id
LEFT JOIN playback_progress p ON p.item_id = i.id AND p.user_id = @user_id
WHERE me.id = @item_id
  AND (@is_admin::bool OR NOT l.restricted
       OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
  AND EXISTS (SELECT 1 FROM media_files f WHERE f.item_id = i.id AND f.missing_since IS NULL)
ORDER BY i.release_date NULLS LAST, i.year NULLS LAST, i.sort_title, i.id;

-- name: ListCollectionMembers :many
-- Every other title of the item's collection, for the administrator editing it.
SELECT i.id, i.kind, i.title, i.year, i.poster_path, i.thumb_path
FROM items me
JOIN items i ON i.collection_id = me.collection_id AND i.id <> me.id
WHERE me.id = @item_id
ORDER BY i.release_date NULLS LAST, i.year NULLS LAST, i.sort_title, i.id;

-- name: SearchCollectionCandidates :many
-- Movies and videos an administrator can add to the item's collection.
SELECT i.id, i.kind, i.title, i.year, i.poster_path, i.thumb_path
FROM items i
WHERE i.parent_id IS NULL AND i.kind IN ('movie', 'video') AND i.id <> @item_id
  AND (search_fold(i.title) ILIKE search_fold(@query) OR search_fold(i.original_title) ILIKE search_fold(@query))
ORDER BY i.sort_title, i.year, i.id
LIMIT @lim;

-- name: CountCollectableItems :one
SELECT count(*)::int FROM items WHERE id = ANY(@ids::uuid[]) AND parent_id IS NULL AND kind IN ('movie', 'video');

-- name: LeaveCollection :exec
-- Takes the items out of their collection, except the ones listed in keep.
UPDATE items SET collection_id = NULL, updated_at = now()
WHERE collection_id = @collection_id AND NOT (id = ANY(@keep::uuid[]));

-- name: JoinCollection :exec
UPDATE items SET collection_id = @collection_id, updated_at = now() WHERE id = ANY(@ids::uuid[]);

-- name: DissolveSingleCollections :exec
-- A collection left with a single title no longer groups anything.
UPDATE items SET collection_id = NULL, updated_at = now()
WHERE collection_id IN (
    SELECT collection_id FROM items WHERE collection_id IS NOT NULL GROUP BY collection_id HAVING count(*) < 2
);

-- name: InheritCollection :exec
-- Keeps the collection of an item merged into another one that has none.
UPDATE items w SET collection_id = l.collection_id
FROM items l
WHERE w.id = @winner AND l.id = @loser AND w.collection_id IS NULL;
