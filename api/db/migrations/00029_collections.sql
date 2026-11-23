-- +goose Up
-- Movies and videos an administrator grouped together (a saga such as Harry Potter); their pages list one another.
-- Items sharing a collection_id form the collection, so an item belongs to one collection at most.
ALTER TABLE items ADD COLUMN collection_id uuid;
CREATE INDEX items_collection_idx ON items (collection_id) WHERE collection_id IS NOT NULL;

-- +goose Down
DROP INDEX items_collection_idx;
ALTER TABLE items DROP COLUMN collection_id;
