-- +goose Up
-- Title and filter searches ignore accents ("amelie" finds "Amélie").
-- unaccent is a trusted extension, so the database owner can create it without being a superuser.
-- It lives in public so that every schema (one per test package) reaches the same copy.
CREATE EXTENSION IF NOT EXISTS unaccent SCHEMA public;

-- unaccent() is only STABLE (its dictionary could change); this IMMUTABLE wrapper pins the dictionary
-- and the schema, so it does not depend on search_path and can be used in indexes later.
-- +goose StatementBegin
CREATE FUNCTION search_fold(text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION search_fold(text);
