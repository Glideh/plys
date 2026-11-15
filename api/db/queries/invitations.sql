-- name: CreateInvitation :one
INSERT INTO invitations (token_hash, pin_hash, locale, created_by, expires_at)
VALUES (@token_hash, @pin_hash, @locale, @created_by, @expires_at)
RETURNING *;

-- name: ListInvitations :many
SELECT * FROM invitations WHERE expires_at > now() ORDER BY created_at;

-- name: DeleteExpiredInvitations :execrows
DELETE FROM invitations WHERE expires_at <= now();

-- name: DeleteInvitation :execrows
DELETE FROM invitations WHERE id = $1;

-- name: GetInvitationByToken :one
SELECT * FROM invitations WHERE token_hash = @token_hash AND expires_at > now();

-- name: GetInvitationByTokenForUpdate :one
SELECT * FROM invitations WHERE token_hash = @token_hash AND expires_at > now() FOR UPDATE;

-- name: RecordInvitationFailure :one
UPDATE invitations SET failed_attempts = failed_attempts + 1 WHERE id = $1 RETURNING failed_attempts;
