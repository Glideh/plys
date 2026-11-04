-- +goose Up
-- A peer is another Plys server this one has been paired with (its id comes from its /server response).
CREATE TABLE peers (
    id         uuid PRIMARY KEY,
    url        text NOT NULL UNIQUE,
    name       text NOT NULL,
    public_key bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE peers;
