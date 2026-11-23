-- name: ListItemIMDbIDs :many
SELECT DISTINCT imdb_id::text FROM items WHERE imdb_id IS NOT NULL AND imdb_id <> '';

-- name: SetIMDbRatings :execrows
-- Applies the ratings read from the IMDb dataset to every item; items whose IMDb ID is missing from it lose theirs.
UPDATE items i SET imdb_rating = r.rating, imdb_votes = r.votes
FROM (
    SELECT x.id, v.rating, v.votes
    FROM items x
    LEFT JOIN (
        SELECT unnest(@imdb_ids::text[]) AS imdb_id, unnest(@ratings::real[]) AS rating, unnest(@votes::int[]) AS votes
    ) v ON v.imdb_id = x.imdb_id
) r
WHERE r.id = i.id
  AND (i.imdb_rating IS DISTINCT FROM r.rating OR i.imdb_votes IS DISTINCT FROM r.votes);
