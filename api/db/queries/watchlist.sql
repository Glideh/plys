-- name: AddToWatchlist :exec
INSERT INTO watchlist (user_id, item_id) VALUES (@user_id, @item_id)
ON CONFLICT (user_id, item_id) DO NOTHING;

-- name: RemoveFromWatchlist :exec
DELETE FROM watchlist WHERE user_id = @user_id AND item_id = @item_id;

-- name: IsInWatchlist :one
SELECT EXISTS (SELECT 1 FROM watchlist WHERE user_id = @user_id AND item_id = @item_id);

-- name: ListWatchlist :many
-- The user's watchlist, most recently added first, limited to the libraries the user may see.
SELECT i.id, i.kind, i.title, i.year, i.poster_path, i.thumb_path, w.added_at
FROM watchlist w
JOIN items i ON i.id = w.item_id
JOIN libraries l ON l.id = i.library_id
WHERE w.user_id = @user_id
  AND (@is_admin::bool OR NOT l.restricted
       OR EXISTS (SELECT 1 FROM library_access a WHERE a.library_id = l.id AND a.user_id = @user_id))
ORDER BY w.added_at DESC, i.id
LIMIT @lim;
