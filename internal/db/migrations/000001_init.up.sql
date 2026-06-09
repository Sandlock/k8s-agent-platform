CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- api_keys stores BYOK keys envelope-encrypted at rest.
-- The plaintext key MUST NOT be stored here.
CREATE TABLE IF NOT EXISTS api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ciphertext BYTEA NOT NULL,
    key_hint   TEXT NOT NULL,  -- last 4 chars of the plaintext key, for UI display only
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sandboxes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    harness      TEXT NOT NULL DEFAULT 'claude-code',
    repo_url     TEXT,
    status       TEXT NOT NULL DEFAULT 'requested',
    provider     TEXT NOT NULL DEFAULT 'kubernetes',
    provider_ref TEXT,               -- namespace/cr-name
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ,

    CONSTRAINT sandboxes_status_check CHECK (
        status IN ('requested','claiming','running','stopping','gone','failed')
    )
);

CREATE INDEX IF NOT EXISTS sandboxes_user_status ON sandboxes(user_id, status);
CREATE INDEX IF NOT EXISTS sessions_token_hash ON sessions(token_hash);
