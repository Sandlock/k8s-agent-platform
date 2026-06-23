---
id: overview
title: How It Works
sidebar_position: 1
---

# Architecture Overview

Sandlock is composed of four components: **control plane**, **supervisor**, **CLI**, and **web dashboard**. They interact with two external systems: the **agent-sandbox controller** (pod lifecycle) and **PostgreSQL** (state).

## Component map

```
  ┌──────────────────────────────────────────────────────────────────┐
  │                        Kubernetes cluster                         │
  │                                                                    │
  │  ┌─────────────────────────────────────────────────────────────┐  │
  │  │   sandlock-system namespace                                  │  │
  │  │                                                              │  │
  │  │  ┌─────────────────────┐    ┌──────────────────────────┐   │  │
  │  │  │   controlplane      │    │   postgres-0             │   │  │
  │  │  │   :8090             │◄──►│   :5432                  │   │  │
  │  │  │                     │    │   users, sessions,       │   │  │
  │  │  │  REST API           │    │   api_keys, sandboxes,   │   │  │
  │  │  │  WebSocket proxy    │    │   agent_snapshots        │   │  │
  │  │  │  Pod reconciler     │    └──────────────────────────┘   │  │
  │  │  └──────────┬──────────┘                                    │  │
  │  │             │ SandboxClaim CRUD                             │  │
  │  │             ▼                                               │  │
  │  │  ┌─────────────────────┐                                    │  │
  │  │  │  agent-sandbox      │                                    │  │
  │  │  │  controller         │                                    │  │
  │  │  └──────────┬──────────┘                                    │  │
  │  └─────────────│─────────────────────────────────────────────┘  │
  │                │ assigns pod from warm pool                       │
  │  ┌─────────────│──────────────────────────────────────────────┐  │
  │  │   sandboxes namespace                ▼                      │  │
  │  │                            ┌─────────────────────┐          │  │
  │  │   NetworkPolicy:           │  pod (supervisor)    │          │  │
  │  │   ingress only from        │                      │          │  │
  │  │   sandlock-system          │  :8080 control       │          │  │
  │  │   on 8080, 8081            │  :8081 PTY bridge    │          │  │
  │  │                            │                      │          │  │
  │  │   egress: HTTPS + DNS only │  harness (claude)    │          │  │
  │  │                            └─────────────────────┘          │  │
  │  └─────────────────────────────────────────────────────────────┘  │
  │                                                                    │
  └──────────────────────────────────────────────────────────────────┘

     Browser / CLI
     ─────────────► Ingress → controlplane:8090
```

## Full request flow: `sandlock create`

### 1. CLI → Control plane

The CLI POSTs to `POST /v1/sandboxes`:

```
POST /v1/sandboxes HTTP/1.1
Authorization: Bearer <token>

{
  "harness": "claude-code",
  "useStoredKey": true,
  "repoUrl": "https://github.com/my-org/my-repo",
  "branch": "main"
}
```

### 2. Control plane claims a pod

The control plane creates a `SandboxClaim` resource:

```yaml
apiVersion: extensions.agents.x-k8s.io/v1alpha1
kind: SandboxClaim
metadata:
  generateName: "sc-"
  namespace: sandboxes
spec:
  templateRef: claude-code
  warmPool: WarmPoolPolicyDefault
  lifecycle:
    shutdownPolicy: Delete
    ttlSecondsAfterFinished: 10
```

It then polls `SandboxClaim.status.sandboxStatus.name` every 2 seconds for up to 120 seconds. Once the agent-sandbox controller assigns a warm pod, `status.sandboxStatus.name` is set to the name of a `Sandbox` resource.

The control plane fetches the `Sandbox` to get its `status.serviceFQDN` — the in-cluster DNS name of the headless Service in front of the pod (e.g. `sb-warmpool-xyz.sandboxes.svc.cluster.local`).

### 3. Key resolution and snapshot lookup

Before calling the supervisor, the control plane:

- Resolves the API key (`anthropicKey` > stored BYOK key > empty)
- Queries `agent_snapshots` for a prior snapshot keyed on `(user_id, repo_url, branch)`. If one exists, it is decrypted and passed to the supervisor.
- Queries `user_skills` for all skills belonging to the caller. These are included in the claim so the supervisor can install them before Claude Code starts.

### 4. Control plane → Supervisor (claim)

```
POST http://sb-warmpool-xyz.sandboxes.svc.cluster.local:8080/claim

{
  "harness": "claude-code",
  "anthropicKey": "sk-ant-...",
  "repoUrl": "https://github.com/my-org/my-repo",
  "branch": "main",
  "sessionSnapshot": <gzip+tar bytes>,
  "callbackUrl": "http://sandlock-controlplane.sandlock-system.svc:8090/v1/internal/snapshots/<id>",
  "skills": [
    { "name": "code-review", "content": "Review the staged diff..." }
  ]
}
```

The supervisor is single-use: it atomically sets a `claimed` flag and rejects any second claim with `409 Conflict`.

### 5. Supervisor launches Claude Code

The supervisor:

1. Writes the snapshot to `~/.claude/` (if present)
2. Writes each skill to `~/.claude/commands/<name>.md` (makes them available as `/<name>` in Claude Code)
3. Runs `git clone --depth=1 <repoUrl> /workspace`
4. Checks out the branch:
   - `git checkout <branch>`
   - If that fails: `git fetch origin <branch> && git checkout <branch>`
   - If that fails: `git checkout -b <branch>` (create locally)
5. Builds the command:
   - Without snapshot: `claude --dangerously-skip-permissions`
   - With snapshot: `claude --dangerously-skip-permissions --continue || claude --dangerously-skip-permissions` (falls back to a fresh start if `--continue` finds no prior conversation in the restored snapshot)
6. Starts Claude Code in a PTY with `ANTHROPIC_API_KEY` and `GITHUB_TOKEN` set
7. The PTY output is teed to a 256 KB scrollback ring buffer and broadcast to all connected WebSocket clients

### 6. Control plane responds to CLI

```json
{
  "sandboxId": "7f3a1b2c-...",
  "attachUrl": "wss://sandlock.example.com/v1/sandboxes/7f3a1b2c-.../tunnel",
  "resumed": true
}
```

### 7. CLI → WebSocket tunnel

The CLI opens a WebSocket connection to `attachUrl?token=<bearer>`. The control plane proxies it to `pod:8081/`. The supervisor replays the scrollback then streams live PTY output.

### 8. Session exit and snapshot

When Claude Code exits:

1. The supervisor tars `~/.claude/` + `~/.claude.json`
2. POSTs the archive to `callbackUrl` (the control plane's internal snapshot endpoint)
3. The control plane encrypts and upserts the snapshot in `agent_snapshots`
4. The supervisor calls `os.Exit(0)`, the pod process ends
5. The control plane's background reconciler (runs every 30 seconds) detects the `SandboxClaim` is gone and marks the sandbox `status = 'gone'` in the database
6. The warm pool controller starts a new pod to replace the one that was consumed

## Reconciler

A background goroutine in the control plane runs every 30 seconds:

1. Fetches all `status = 'running'` sandboxes from the database
2. Lists all `SandboxClaims` in the cluster
3. For any database sandbox whose `provider_ref` no longer has a corresponding claim, sets `status = 'gone'`

This handles cases where a pod dies unexpectedly or the `SandboxClaim` is deleted externally.
