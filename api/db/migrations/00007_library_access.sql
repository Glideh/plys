-- +goose Up
-- Unrestricted libraries are visible to every user; restricted ones only to the listed users. Admins see all.
ALTER TABLE libraries ADD COLUMN restricted boolean NOT NULL DEFAULT false;

CREATE TABLE library_access (
    library_id uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (library_id, user_id)
);

CREATE INDEX library_access_user_idx ON library_access (user_id);

-- +goose Down
DROP TABLE library_access;
ALTER TABLE libraries DROP COLUMN restricted;
