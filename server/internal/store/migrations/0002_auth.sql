-- M2 login. NULL password_hash means the account cannot log in (system or test accounts).
ALTER TABLE accounts ADD COLUMN password_hash text;

-- Only the SHA-256 of a session token is stored; the token itself is never persisted.
CREATE TABLE sessions (
    token_sha256 bytea PRIMARY KEY,
    account_id   bigint NOT NULL REFERENCES accounts (id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_account ON sessions (account_id);
