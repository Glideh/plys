-- A title of the watchlist is the item that was added or any other top-level copy of the same title, that is the same
-- kind and TMDB ID, whichever library holds it.

-- name: AddToWatchlist :exec
INSERT INTO watchlist (user_id, item_id) VALUES (@user_id, @item_id)
ON CONFLICT (user_id, item_id) DO NOTHING;

-- name: RemoveFromWatchlist :exec
-- Removes the item and every other copy of its title.
DELETE FROM watchlist w
USING items t, items i
WHERE w.user_id = @user_id AND t.id = @item_id AND i.id = w.item_id
  AND (i.id = t.id
       OR (t.tmdb_id IS NOT NULL AND i.parent_id IS NULL AND i.kind = t.kind AND i.tmdb_id = t.tmdb_id));

-- name: RemoveTitleFromWatchlist :exec
-- Removes every copy of a title.
DELETE FROM watchlist w
USING items i
WHERE w.user_id = @user_id AND i.id = w.item_id
  AND i.parent_id IS NULL AND i.kind = @item_kind AND i.tmdb_id = @tmdb_id::int;

-- name: IsInWatchlist :one
-- Whether the item or another copy of its title is in the watchlist.
SELECT EXISTS (
    SELECT 1
    FROM watchlist w
    JOIN items i ON i.id = w.item_id
    JOIN items t ON t.id = @item_id
    WHERE w.user_id = @user_id
      AND (i.id = t.id
           OR (t.tmdb_id IS NOT NULL AND i.parent_id IS NULL AND i.kind = t.kind AND i.tmdb_id = t.tmdb_id))
);

-- name: IsTitleInWatchlist :one
-- Whether a copy of the title is in the watchlist.
SELECT EXISTS (
    SELECT 1
    FROM watchlist w
    JOIN items i ON i.id = w.item_id
    WHERE w.user_id = @user_id AND i.parent_id IS NULL AND i.kind = @item_kind AND i.tmdb_id = @tmdb_id::int
);

-- name: ListWatchlist :many
-- The user's watchlist, most recently added first, limited to the libraries the user may see.
SELECT i.id, i.kind, i.title, i.year, i.tmdb_id, i.poster_path, i.thumb_path, w.added_at
FROM watchlist w
JOIN items i ON i.id = w.item_id
JOIN libraries l ON l.id = i.library_id
WHERE w.user_id = @user_id
  AND (@is_admin::bool OR NOT l.restricted
       OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
ORDER BY w.added_at DESC, i.id
LIMIT @lim;
