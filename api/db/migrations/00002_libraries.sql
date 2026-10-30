-- +goose Up
CREATE TABLE libraries (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    kind       text NOT NULL CHECK (kind IN ('movies', 'series', 'other')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX libraries_name_lower_idx ON libraries (lower(name));

-- Paths are stored after symlink resolution; nesting across libraries is rejected by the API.
CREATE TABLE library_paths (
    library_id uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path       text NOT NULL,
    PRIMARY KEY (library_id, path)
);

CREATE UNIQUE INDEX library_paths_path_idx ON library_paths (path);

-- +goose Down
DROP TABLE library_paths;
DROP TABLE libraries;
