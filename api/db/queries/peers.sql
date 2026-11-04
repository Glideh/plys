-- name: ListPeers :many
SELECT * FROM peers ORDER BY name, url;

-- name: GetPeer :one
SELECT * FROM peers WHERE id = $1;

-- name: InsertPeer :one
INSERT INTO peers (id, url, name, public_key)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdatePeerURL :one
UPDATE peers SET url = $2 WHERE id = $1 RETURNING *;

-- name: ClaimPeerUserSync :execrows
-- Lets only one request sync a peer per interval, concurrent listings included.
UPDATE peers SET users_sync_attempted_at = @attempted_at
WHERE id = @id AND (users_sync_attempted_at IS NULL OR users_sync_attempted_at < @cutoff);

-- name: PeerAllowsOrigin :one
SELECT EXISTS (SELECT 1 FROM peers WHERE lower(url) = lower(@origin));

-- name: DeletePeer :execrows
DELETE FROM peers WHERE id = $1;
