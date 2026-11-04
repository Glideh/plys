-- +goose Up
ALTER TABLE items
    ADD COLUMN genre_ids integer[] NOT NULL DEFAULT '{}',
    ADD COLUMN actor_ids integer[] NOT NULL DEFAULT '{}',
    ADD COLUMN creator_ids integer[] NOT NULL DEFAULT '{}',
    ADD COLUMN keyword_ids integer[] NOT NULL DEFAULT '{}',
    ADD COLUMN original_language text,
    ADD COLUMN countries text[] NOT NULL DEFAULT '{}',
    ADD COLUMN vote_count integer,
    ADD COLUMN filter_labels jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN enrichment_version integer NOT NULL DEFAULT 0;
CREATE INDEX items_genre_ids_idx ON items USING gin (genre_ids);
CREATE INDEX items_actor_ids_idx ON items USING gin (actor_ids);
CREATE INDEX items_creator_ids_idx ON items USING gin (creator_ids);
CREATE INDEX items_keyword_ids_idx ON items USING gin (keyword_ids);

-- +goose Down
DROP INDEX items_keyword_ids_idx;
DROP INDEX items_creator_ids_idx;
DROP INDEX items_actor_ids_idx;
DROP INDEX items_genre_ids_idx;
ALTER TABLE items
    DROP COLUMN enrichment_version,
    DROP COLUMN filter_labels,
    DROP COLUMN vote_count,
    DROP COLUMN countries,
    DROP COLUMN original_language,
    DROP COLUMN keyword_ids,
    DROP COLUMN creator_ids,
    DROP COLUMN actor_ids,
    DROP COLUMN genre_ids;