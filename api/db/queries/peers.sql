-- name: ListPeers :many
SELECT * FROM peers ORDER BY lower(coalesce(alias, name)), url;

-- name: GetPeer :one
SELECT * FROM peers WHERE id = $1;

-- name: InsertPeer :one
INSERT INTO peers (id, url, name, public_key)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdatePeerURL :one
UPDATE peers SET url = $2 WHERE id = $1 RETURNING *;

-- name: UpdatePeerVersion :one
UPDATE peers SET version = $2, version_checked_at = now() WHERE id = $1 RETURNING *;

-- name: ClaimPeerUserSync :execrows
-- Lets only one request sync a peer per interval, concurrent listings included.
UPDATE peers SET users_sync_attempted_at = @attempted_at
WHERE id = @id AND (users_sync_attempted_at IS NULL OR users_sync_attempted_at < @cutoff);

-- name: PeerAllowsOrigin :one
SELECT EXISTS (SELECT 1 FROM peers WHERE lower(url) = lower(@origin));

-- name: DeletePeer :execrows
DELETE FROM peers WHERE id = $1;

-- name: SetPeerPaused :one
UPDATE peers SET paused = $2 WHERE id = $1 RETURNING *;

-- name: RecordPeerFailure :one
-- Failures closer than @spacing_cutoff to the previous one are the same outage seen by concurrent requests.
UPDATE peers SET
  consecutive_failures = consecutive_failures + 1,
  last_failure_at = @now,
  offline_since = CASE WHEN consecutive_failures + 1 >= @threshold::int THEN coalesce(offline_since, @now) ELSE offline_since END
WHERE id = @id AND (last_failure_at IS NULL OR last_failure_at < @spacing_cutoff)
RETURNING *;

-- name: RecordPeerSuccess :exec
UPDATE peers SET consecutive_failures = 0, last_failure_at = NULL, offline_since = NULL, last_seen_at = now()
WHERE id = $1;

-- name: SetPeerAlias :one
UPDATE peers SET alias = @alias WHERE id = @id RETURNING *;
