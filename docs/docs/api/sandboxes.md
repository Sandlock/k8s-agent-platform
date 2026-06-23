---
id: sandboxes
title: Sandboxes API
sidebar_position: 3
---

# Sandboxes

## `POST /v1/sandboxes`

Create (claim) a new sandbox.

**Auth required:** Yes

### Request

```json
{
  "harness": "claude-code",
  "anthropicKey": "sk-ant-...",
  "useStoredKey": false,
  "repoUrl": "https://github.com/my-org/my-repo",
  "branch": "feature/my-feature",
  "githubToken": "ghp_...",
  "noResume": false
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `harness` | string | No | Agent harness to run. Default: `claude-code`. |
| `anthropicKey` | string | No | Inject an API key inline. Takes priority over `useStoredKey`. |
| `useStoredKey` | bool | No | Decrypt and use the key stored via `POST /v1/keys`. |
| `repoUrl` | string | No | Git repo URL to shallow-clone. Normalized (trailing `/` and `.git` suffix stripped) before storage. |
| `branch` | string | No | Branch to clone and check out. Passed as `--branch` to the clone; created locally if it doesn't exist on the remote. |
| `githubToken` | string | No | GitHub token for cloning private repos with `gh repo clone`. |
| `noResume` | bool | No | If `true`, ignore any existing snapshot for this repo+branch. Default: `false`. |

### Response `201 Created`

```json
{
  "sandboxId": "7f3a1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c",
  "attachUrl": "wss://sandlock.example.com/v1/sandboxes/7f3a1b2c-.../tunnel",
  "resumed": true
}
```

| Field | Type | Description |
|---|---|---|
| `sandboxId` | UUID | Identifier for this sandbox. |
| `attachUrl` | string | WebSocket URL for the terminal. Append `?token=<bearer>` to authenticate. |
| `resumed` | bool | Whether a session snapshot was found and will be restored. |

### Errors

| Status | Condition |
|---|---|
| `400` | Invalid harness name, or `useStoredKey` requested but no key is stored |
| `504` | Warm pool had no available pod within 120 seconds |

### Warm pool wait

The control plane polls for a ready pod every 2 seconds for up to 120 seconds. If no pod is available, the request returns a timeout error. Increase `pool.targetReady` in your Helm values if this happens frequently.

---

## `GET /v1/sandboxes`

List sandboxes owned by the authenticated user.

**Auth required:** Yes

### Response `200 OK`

```json
[
  {
    "id": "7f3a1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c",
    "harness": "claude-code",
    "status": "running",
    "providerRef": "claim/sandboxes/sc-abc123",
    "createdAt": "2026-06-22T13:24:05Z",
    "repoUrl": "https://github.com/my-org/my-repo",
    "branch": "main"
  }
]
```

`repoUrl` and `branch` are omitted from the response when the sandbox was created without a repository.

Sandboxes with `status = 'gone'` are excluded.

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Sandbox identifier. |
| `harness` | string | Agent harness running in the pod. |
| `status` | string | Current status: `running` or `failed`. |
| `providerRef` | string | Internal Kubernetes claim reference. |
| `createdAt` | timestamp | When the sandbox was created. |
| `repoUrl` | string | Git repository URL (omitted if no repo was cloned). |
| `branch` | string | Branch checked out inside the pod (omitted if no repo was cloned). |

---

## `GET /v1/sandboxes/{id}`

Get a single sandbox.

**Auth required:** Yes

### Response `200 OK`

Same shape as a single item from `GET /v1/sandboxes`.

### Errors

| Status | Condition |
|---|---|
| `404` | Sandbox not found or not owned by you |

---

## `DELETE /v1/sandboxes/{id}`

Stop and destroy a sandbox.

**Auth required:** Yes

### Behavior

1. If the sandbox has a `repo_url`, the control plane fetches the session snapshot from the pod (`GET pod:8080/snapshot`) and stores it encrypted in the database.
2. Updates `status = 'gone'` in the database.
3. Deletes the `SandboxClaim` Kubernetes resource (pod is destroyed by agent-sandbox).

### Response `204 No Content`

### Errors

| Status | Condition |
|---|---|
| `404` | Sandbox not found or not owned by you |

---

## `GET /v1/sandboxes/{id}/tunnel`

Open a WebSocket terminal connection to the sandbox PTY.

**Auth required:** Yes (bearer token via `Authorization` header or `?token=` query param)

### WebSocket protocol

The connection is established with HTTP Upgrade. The control plane proxies all frames between the client and the supervisor's PTY bridge (`:8081`).

**Frame types:**

| Direction | Type | Payload |
|---|---|---|
| Server → Client | Binary | Raw PTY output (terminal bytes) |
| Client → Server | Binary | Keystrokes / terminal input |
| Client → Server | Text | Resize message: `{"rows": 40, "cols": 120}` |

On connect, the supervisor sends the scrollback buffer (up to 256 KB of prior output) before streaming live PTY output.

### Errors

| Status | Condition |
|---|---|
| `404` | Sandbox not found or not owned by you |
| `400` | Not a WebSocket upgrade request |

:::note
`GET /v1/sandboxes/{id}/terminal` is a deprecated alias for `/tunnel` and behaves identically.
:::

---

## `POST /v1/internal/snapshots/{id}`

*(Internal — called by the supervisor, not user-facing)*

Accept and store a session snapshot.

**Auth required:** No (protected by Kubernetes NetworkPolicy — only callable from within the cluster)

**Content-Type:** `application/octet-stream`

**Body:** Raw gzip+tar binary (max 15 MB) of the `~/.claude/` directory.

The control plane:
1. Looks up `(user_id, repo_url, branch)` from the sandbox record
2. Encrypts the binary with AES-256-GCM
3. Upserts into `agent_snapshots` (unique key: `user_id + repo_url + branch`)

### Response `200 OK`
