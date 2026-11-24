-- +goose Up
-- A sign-in started on a TV and approved from a signed-in phone. The TV keeps a secret device code and polls with it;
-- the user compares the short user code shown on both screens. Only the device code hash is stored. The row is deleted
-- when the TV collects its session, so a request opens at most one session.
CREATE TABLE device_logins (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code_hash bytea NOT NULL UNIQUE,
    user_code        text NOT NULL UNIQUE,
    device_name      text NOT NULL,
    user_id          uuid REFERENCES users (id) ON DELETE CASCADE,
    approved_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    expires_at       timestamptz NOT NULL
);

-- +goose Down
DROP TABLE device_logins;
