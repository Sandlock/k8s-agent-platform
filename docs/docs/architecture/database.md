---
id: database
title: Database Schema
sidebar_position: 4
---

# Database Schema

Sandlock uses PostgreSQL, managed with [golang-migrate](https://github.com/golang-migrate/migrate). Migrations run automatically at control plane startup.

## Tables

### `users`

Stores user accounts.

```sql
CREATE TABLE users (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username             TEXT UNIQUE NOT NULL,
  password_hash        TEXT NOT NULL,
  is_admin             BOOLEAN NOT NULL DEFAULT false,
  must_change_password BOOLEAN NOT NULL DEFAULT false,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

| Column | Description |
|---|---|
| `id` | UUID primary key, auto-generated |
| `username` | Unique login name |
| `password_hash` | argon2id hash: `hex(salt) + "$" + hex(hash)` |
| `is_admin` | Grants access to `GET/POST/DELETE /v1/users` |
| `must_change_password` | Set on new accounts; cleared after `PUT /v1/auth/password` |

---

### `sessions`

Active bearer token sessions.

```sql
CREATE TABLE sessions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_token_hash ON sessions (token_hash);
```

| Column | Description |
|---|---|
| `token_hash` | `SHA-256(raw_token)` — plaintext token is never stored |
| `expires_at` | 24 hours after creation |

Deleting a user cascades to all their sessions, immediately invalidating them.

---

### `api_keys`

Encrypted Anthropic API keys, one per user.

```sql
CREATE TABLE api_keys (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  ciphertext BYTEA NOT NULL,
  key_hint   TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

| Column | Description |
|---|---|
| `user_id` | `UNIQUE` — one stored key per user |
| `ciphertext` | `nonce (12 bytes) \|\| AES-256-GCM ciphertext` |
| `key_hint` | Last 4 characters of the plaintext key (shown in the UI) |

---

### `sandboxes`

Records of every sandbox claim.

```sql
CREATE TABLE sandboxes (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
  harness      TEXT NOT NULL DEFAULT 'claude-code',
  repo_url     TEXT NOT NULL DEFAULT '',
  branch       TEXT NOT NULL DEFAULT '',
  status       TEXT NOT NULL CHECK (status IN (
                  'requested', 'claiming', 'running',
                  'stopping', 'gone', 'failed'
               )),
  provider     TEXT NOT NULL DEFAULT 'kubernetes',
  provider_ref TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at   TIMESTAMPTZ
);

CREATE INDEX sandboxes_user_status ON sandboxes (user_id, status);
```

| Column | Description |
|---|---|
| `repo_url` | Normalized clone URL (no `.git` suffix, no trailing `/`) |
| `branch` | Git branch name (empty string if no branch specified) |
| `status` | Lifecycle state — see below |
| `provider_ref` | `"claim/<namespace>/<name>"` — pointer to the `SandboxClaim` resource |

**Status values:**

| Status | Meaning |
|---|---|
| `requested` | Create request received, claim not yet issued |
| `claiming` | `SandboxClaim` created, waiting for a warm pod |
| `running` | Supervisor received claim; Claude Code is running |
| `stopping` | `DELETE /v1/sandboxes/{id}` in progress |
| `gone` | Pod has been deleted; terminal state |
| `failed` | Error during claim or launch |

---

### `agent_snapshots`

Encrypted `~/.claude/` archives for session persistence.

```sql
CREATE TABLE agent_snapshots (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  repo_url    TEXT NOT NULL,
  branch      TEXT NOT NULL DEFAULT '',
  snapshot    BYTEA NOT NULL,
  snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT agent_snapshots_user_repo_branch_key
    UNIQUE (user_id, repo_url, branch)
);
```

| Column | Description |
|---|---|
| `repo_url` | Normalized repo URL (same normalization as `sandboxes.repo_url`) |
| `branch` | Branch the snapshot was taken from |
| `snapshot` | `nonce (12 bytes) \|\| AES-256-GCM(gzip+tar of ~/.claude/)` |
| `snapshot_at` | Updated on every write |

The `UNIQUE` constraint on `(user_id, repo_url, branch)` means only one snapshot per user per repo per branch is retained — new snapshots replace old ones via `INSERT ... ON CONFLICT DO UPDATE`.

---

## Migrations

| Number | File | Change |
|---|---|---|
| `000001` | `init.up.sql` | Create `users`, `sessions`, `api_keys`, `sandboxes` |
| `000002` | `users_admin.up.sql` | Add `is_admin`, `must_change_password` to `users` |
| `000003` | `agent_snapshots.up.sql` | Create `agent_snapshots` with `(user_id, repo_url)` unique key |
| `000004` | `branch.up.sql` | Add `branch` to `sandboxes` and `agent_snapshots`; update unique key to include `branch` |

Migrations are embedded in the control plane binary and run automatically on startup using `golang-migrate`. Down migrations are provided for each step but are not run automatically.
