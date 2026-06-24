CREATE TABLE user_mcp_servers (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name                      TEXT NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,63}$'),
    server_type               TEXT NOT NULL DEFAULT 'http'
                                  CHECK (server_type IN ('http', 'sse', 'stdio')),
    url                       TEXT,
    command                   TEXT,
    args                      JSONB NOT NULL DEFAULT '[]',
    env_vars                  JSONB NOT NULL DEFAULT '{}',
    secret_env_ciphertext     BYTEA,
    headers                   JSONB NOT NULL DEFAULT '{}',
    secret_headers_ciphertext BYTEA,
    display_name              TEXT NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);
