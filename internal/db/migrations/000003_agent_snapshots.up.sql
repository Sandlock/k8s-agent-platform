CREATE TABLE agent_snapshots (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_url    TEXT NOT NULL,
    snapshot    BYTEA NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, repo_url)
);
