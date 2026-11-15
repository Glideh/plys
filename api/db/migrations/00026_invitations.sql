-- +goose Up
-- An invitation lets someone create their own account. The link carries a secret token and the admin hands over a
-- short PIN through another channel. Only hashes are stored; the PIN hash is keyed by the token, so that it cannot be
-- brute-forced without the link.
CREATE TABLE invitations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash      bytea NOT NULL UNIQUE,
    pin_hash        bytea NOT NULL,
    locale          text NOT NULL,
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    failed_attempts integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL
);

-- +goose Down
DROP TABLE invitations;
