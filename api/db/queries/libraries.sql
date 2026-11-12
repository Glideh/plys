-- name: LockLibraries :exec
-- Serializes path overlap checks across concurrent library mutations.
SELECT pg_advisory_xact_lock(7461002);

-- name: CreateLibrary :one
INSERT INTO libraries (name, kind, metadata_language) VALUES ($1, $2, $3) RETURNING *;

-- name: RenameLibrary :one
UPDATE libraries SET name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: SetLibraryMetadataLanguage :one
UPDATE libraries SET metadata_language = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteLibrary :execrows
DELETE FROM libraries WHERE id = $1;

-- name: GetLibrary :one
SELECT * FROM libraries WHERE id = $1;

-- name: ListLibraries :many
SELECT * FROM libraries ORDER BY lower(name);

-- name: ListAllLibraryPaths :many
SELECT library_id, path FROM library_paths ORDER BY path;

-- name: ListLibraryPaths :many
SELECT path FROM library_paths WHERE library_id = $1 ORDER BY path;

-- name: AddLibraryPath :exec
INSERT INTO library_paths (library_id, path) VALUES ($1, $2);

-- name: ClearLibraryPaths :exec
DELETE FROM library_paths WHERE library_id = $1;
