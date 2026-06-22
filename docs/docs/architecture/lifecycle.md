---
id: lifecycle
title: Sandbox Lifecycle
sidebar_position: 2
---

# Sandbox Lifecycle

## Pod states

```
  ┌──────────┐     claim      ┌──────────┐     harness exits   ┌──────────┐
  │   Idle   │ ─────────────► │  Claimed │ ──────────────────► │   Gone   │
  │ (warm)   │                │ (running)│                      │ (deleted)│
  └──────────┘                └──────────┘                      └──────────┘
       ▲                                                              │
       │                                                              │
       └──────────────── warm pool replenishes ◄─────────────────────┘
```

### Idle (warm pool)

The agent-sandbox warm pool controller maintains a configurable number of pre-warmed pods (`pool.targetReady`, default: 1). Each pod runs the supervisor binary, which listens on `:8080` for a claim request and `:8081` for WebSocket terminal connections.

Pre-warming means that when a user runs `sandlock create`, the pod is already running and only needs to receive the claim — no cold-start delay.

### Claimed

When the control plane creates a `SandboxClaim`, the agent-sandbox controller atomically assigns one warm pod by creating a `Sandbox` resource linking the claim to the pod. A headless Kubernetes Service is created (`sb-<name>.sandboxes.svc.cluster.local`) for in-cluster routing.

The supervisor's `POST /claim` handler atomically sets a `claimed` bool. A second claim returns `409 Conflict`. This prevents races even if two control plane replicas try to claim the same pod simultaneously.

### Gone

When a sandbox is stopped (explicitly via `DELETE /v1/sandboxes/{id}`, or when the Claude Code process exits and the pod is cleaned up), the `SandboxClaim` is deleted. The agent-sandbox controller cascades this to pod deletion and Service deletion.

The control plane reconciler marks the database record `status = 'gone'` within 30 seconds.

---

## Session snapshots

Snapshots allow Claude Code sessions to persist across pod restarts. The state captured is the `~/.claude/` directory, which contains:

- Claude Code configuration and preferences
- Conversation history
- Any persistent context Claude has accumulated

### Snapshot creation

A snapshot is created in two situations:

1. **On explicit stop** (`DELETE /v1/sandboxes/{id}`): The control plane proactively calls `GET pod:8080/snapshot` to fetch the snapshot before deleting the claim.
2. **On harness exit**: When Claude Code exits normally, the supervisor automatically tars `~/.claude/` + `~/.claude.json` and POSTs the archive to the control plane's internal callback URL.

### Snapshot storage

```
POST /v1/internal/snapshots/{sandboxId}
Content-Type: application/octet-stream
Body: gzip+tar of ~/.claude/

→ Control plane:
    1. Decrypts? No — data arrives unencrypted from the pod
    2. Encrypts with AES-256-GCM(MASTER_KEY, nonce, data)
    3. UPSERT into agent_snapshots (user_id, repo_url, branch)
       — replaces any prior snapshot for the same (user, repo, branch) tuple
```

Maximum snapshot size: **15 MB**.

### Snapshot restoration

When `sandlock create` is called with a `repo_url` and `branch`:

1. The control plane queries `agent_snapshots` for a row matching `(user_id, repo_url, branch)`
2. If found, the snapshot bytes are decrypted with `AES-256-GCM.Open(MASTER_KEY, stored_bytes)`
3. The decrypted archive is included in the claim request to the supervisor
4. The supervisor extracts it to `~/` before launching Claude Code
5. Claude Code is started with `--continue` to resume the conversation

### Bypassing restoration

Use `sandlock create --no-resume` to start fresh and ignore any existing snapshot.

### Snapshot key

Snapshots are keyed on `(user_id, repo_url, branch)`. This means:

- Different users have separate snapshots for the same repo
- Different branches of the same repo have separate snapshots
- Only one snapshot per `(user, repo, branch)` is kept — the most recent overwrites the previous

---

## Warm pool configuration

| Helm value | Default | Description |
|---|---|---|
| `pool.targetReady` | `1` | Number of pods to keep pre-warmed and ready to claim |
| `pool.maxTotal` | `20` | Hard cap on the total number of sandbox pods across all states |

Increase `pool.targetReady` for teams with many concurrent users. The pool controller will start replacement pods immediately after each claim.

If all pods are claimed and `maxTotal` is reached, new `sandlock create` calls will wait up to 120 seconds for a pod to become available.
