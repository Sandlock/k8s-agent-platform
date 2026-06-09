# Sandlock — MVP Implementation Spec

> Hand-off document for an AI coding agent. Read the **Summary** first, then build in the
> order given in **Build order & milestones**. Every "MUST" is a hard requirement; every
> "SHOULD" is a strong default you may deviate from only with a stated reason.

---

## Summary (read this first)

Sandlock is developer tooling that runs coding agents (Claude Code first, other harnesses
later) inside isolated, ephemeral Kubernetes pods. Users never touch the cluster: they
interact through a CLI and a web dashboard, both of which talk only to a **control plane**
that holds the cluster credentials. A **published domain + gateway** proxies live
connections from the user's browser or CLI into the pod. A **warm pool** of pre-started,
generic pods makes attaching feel instant; pods are **single-use** and destroyed after one
session.

Five decisions are fixed and shape everything below:

1. **BYOK** — the user supplies their own Anthropic API key. Sandlock never pays for
   inference and never bakes the key into an image or a Kubernetes Secret.
2. **Kubernetes first, pluggable later** — implement a Kubernetes pod provider behind a
   narrow `Provider` interface so other backends (Firecracker, Fly Machines, Fargate) can be
   added without touching the control plane.
3. **No workspace persistence** — pods are ephemeral. The workspace is set up at claim time
   (optional shallow git clone) and discarded when the pod dies.
4. **Warm pool** — the control plane keeps N pods in a `Ready` state; a claim assigns one to
   a user, injects the key in memory, launches the harness, then warms a replacement.
5. **Two-tier secrets** — platform secrets are normal Kubernetes Secrets, mounted at pod
   creation and reconciled by the operator. The user's BYOK key is *never* a Kubernetes
   Secret: it is decrypted in control-plane memory and pushed over an authenticated channel
   into the already-running pod at claim time, living only in pod RAM for one session.

Recommended language for all backend components: **Go** (idiomatic for
Kubernetes, single static binaries, one language across control plane / operator / supervisor
/ CLI). Web dashboard: **React + TypeScript**.

---

## 1. Goals and non-goals

### In scope for MVP
- User auth: username + password only (session tokens; CLI logs in with the same credentials).
- Create a sandbox running Claude Code in an isolated pod via the CLI.
- Warm pool of generic agent pods, claimed on demand.
- BYOK key injection at claim time, in memory only.
- Connect to a running agent two ways: (a) browser terminal via the dashboard, (b) local
  tunnel via the CLI.
- Web dashboard listing the user's running agents with open / stop controls.
- Single Kubernetes provider behind a `Provider` interface.
- Single-use, ephemeral pods with TTL and idle teardown.

### Explicitly out of scope for MVP (design for, do not build)
- Workspace persistence / snapshots.
- Non-Kubernetes providers (the interface exists; only the K8s impl ships).
- Team / org accounts, RBAC beyond "a user owns their own sandboxes".
- Billing / metering (BYOK means no inference cost to meter yet).
- Multi-cluster / BYO-cluster enterprise mode.

---

## 2. System components

| Component | Responsibility | Suggested tech |
|---|---|---|
| Control plane API | Public API for CLI + UI; auth; sandbox lifecycle; holds K8s creds; pool manager; secret handoff | Go HTTP service |
| Sandbox operator | Reconciles `Sandbox` custom resources into real pods; maintains pool size | Go + controller-runtime / kubebuilder |
| Gateway | TLS termination at the published domain; validates session token; proxies websocket/TCP to the right pod's supervisor | Go reverse/ws proxy (MAY be part of the control-plane binary for MVP) |
| Agent supervisor | Runs inside every pod; idles while warm; on claim receives key+config, clones repo, launches harness; exposes terminal bridge + control channel | Go static binary in the base image |
| CLI | `login`, `create`, `list`, `attach`, `stop`; opens local tunnel | Go single binary |
| Web dashboard | Lists running agents; open browser terminal; stop | React + TypeScript |
| Datastore | Users, sessions, sandbox records, pool bookkeeping | PostgreSQL |

The trust boundary is the control plane + operator. Only those hold cluster credentials.
Users receive short-lived, narrowly-scoped session tokens and **never** a kubeconfig.

---

## 3. Data model

### 3.1 Persistent (PostgreSQL)

```
users
  id            uuid pk
  username      text unique
  password_hash text             -- argon2id (preferred) or bcrypt; never store plaintext
  created_at    timestamptz

api_keys                         -- BYOK key storage (optional convenience)
  id            uuid pk
  user_id       uuid fk -> users
  ciphertext    bytea            -- envelope-encrypted; KMS-wrapped DEK
  key_hint      text             -- last 4 chars only, for UI
  created_at    timestamptz
  -- NOTE: plaintext key MUST NOT be stored. Decrypt only in memory at claim.

sessions                         -- auth sessions for CLI + UI
  id            uuid pk
  user_id       uuid fk
  token_hash    text             -- store hash, never the raw token
  expires_at    timestamptz

sandboxes                        -- one row per user-facing sandbox/agent
  id            uuid pk
  user_id       uuid fk
  harness       text             -- "claude-code" for MVP
  repo_url      text null
  status        text             -- requested|claiming|running|stopping|gone|failed
  provider      text             -- "kubernetes"
  provider_ref  text             -- opaque handle (e.g. namespace/pod-name)
  created_at    timestamptz
  expires_at    timestamptz      -- TTL
```

### 3.2 Kubernetes `Sandbox` custom resource

The operator reconciles this. `Sandbox` objects describe **desired** state; the BYOK key is
never part of spec or status.

```yaml
apiVersion: sandlock.dev/v1alpha1
kind: Sandbox
spec:
  harness: claude-code
  pool: true                # true = part of the warm pool (generic, unclaimed)
  resources:
    cpu: "1"
    memory: 2Gi
  ttlSeconds: 3600
  idleTimeoutSeconds: 900
status:
  phase: Warming|Ready|Claimed|Recycling|Failed
  podName: ""
  claimedBy: ""             # user id, set on claim; never the key
  lastHeartbeat: ""
```

A separate `SandboxPool` CR (or a single config object) holds the target Ready count so the
operator can keep the pool topped up:

```yaml
apiVersion: sandlock.dev/v1alpha1
kind: SandboxPool
spec:
  harness: claude-code
  targetReady: 5
  maxTotal: 50
  podTtlSeconds: 3600
```

---

## 4. The Provider interface (the portability seam)

There are **two** seams. Keep them separate.

**Seam A — provisioning** (differs per backend). Narrow interface; the K8s impl is the only
one shipped:

```go
type Provider interface {
    // Provision creates an unclaimed sandbox (used both for the pool and cold fallback).
    Provision(ctx, SandboxSpec) (Handle, error)
    Status(ctx, Handle) (Phase, error)
    Destroy(ctx, Handle) error
    List(ctx) ([]Handle, error)   // for reconciliation / drift detection
}
```

**Seam B — connection** (identical across backends). This is the agent supervisor's protocol.
Because every pod runs the same supervisor exposing the same two channels, the gateway, the
CLI tunnel, and the claim/key-injection logic never need to know which provider is underneath.
**Do not** leak Kubernetes specifics into the gateway or CLI.

The pool manager lives in the control plane **above** the `Provider` interface, so any future
provider inherits warm pooling for free. A provider implements only the four provisioning
methods plus shipping the supervisor in its base image.

---

## 5. Agent supervisor protocol

The supervisor is a small Go binary that is PID 1 (or supervises) inside every pod.

State machine inside the pod:
- **Idle (warm):** supervisor up, no harness running, control channel listening. The harness
  binary/image is present but not launched (it has no key yet).
- **Claimed:** received claim payload, optionally cloned repo, launched harness with key in
  env, terminal bridge active.

Two channels:

1. **Control channel** (control plane → supervisor). Authenticated; reachable **only** from
   the control plane (enforce with a NetworkPolicy). Messages:
   - `Claim { anthropicKey, harness, repoUrl?, env? }` → supervisor sets key in harness env
     **in memory**, shallow-clones repo if provided, launches harness, replies `Attached`.
   - `Stop {}` → supervisor signals teardown.
   - `Heartbeat {}` ↔ liveness/idle reporting.
2. **Terminal bridge** (gateway → supervisor). Websocket ↔ PTY of the harness process
   (the ttyd/gotty pattern). This is what both the browser terminal and the CLI tunnel ride.

Hard rules for the supervisor:
- The BYOK key MUST live only in process memory. Never write it to disk, never log it, never
  pass it as a command-line argument (argv is world-readable via `/proc`); set it as an env
  var on the child process only.
- On `Stop`, session end, or TTL expiry, the pod is destroyed. The supervisor MUST NOT accept
  a second `Claim` — pods are single-use.

---

## 6. Control plane API (HTTP/JSON)

All endpoints require a valid session token except the auth endpoints. A user may only see
and act on their own sandboxes.

```
POST /v1/auth/register              -> create account; body {username, password}
POST /v1/auth/login                 -> {username, password} -> session token (UI + CLI)
POST /v1/auth/logout                -> invalidate the current session token

POST /v1/keys                       -> store BYOK key (envelope-encrypted); body {anthropicKey}
DELETE /v1/keys                     -> forget stored key

POST /v1/sandboxes                  -> create+claim a sandbox
        body { harness, repoUrl?, useStoredKey | anthropicKey }
        behavior: claim a Ready pod from the pool (atomic); if none, cold-provision.
        returns { sandboxId, attachUrl }
GET  /v1/sandboxes                  -> list caller's sandboxes
GET  /v1/sandboxes/{id}             -> status
DELETE /v1/sandboxes/{id}           -> stop + destroy

# Connection (proxied by the gateway, token-scoped to one sandbox):
WS   /v1/sandboxes/{id}/terminal    -> browser terminal bridge
WS   /v1/sandboxes/{id}/tunnel      -> CLI local-tunnel transport
```

The session token presented on the connection endpoints MUST be scoped to a single sandbox
owned by the caller; the gateway rejects any mismatch before proxying.

---

## 7. Key flows

### 7.1 Claim (the core flow)
1. `POST /v1/sandboxes`. Control plane atomically claims a `Ready` pod (optimistic label
   update guarded by `resourceVersion`, **or** a DB transaction — pick one as the single
   source of truth). If the pool is empty, cold-provision a pod (slower, never fails).
2. Control plane decrypts the BYOK key into its own memory (or uses the key from the request).
3. Control plane opens the control channel to that pod's supervisor and sends `Claim` with
   key + harness + optional repo.
4. Supervisor clones repo (shallow), launches harness with key in env, replies `Attached`.
5. Control plane marks the `Sandbox` `Claimed`, records ownership, returns `attachUrl`.
6. Operator/pool manager warms a replacement to restore `targetReady`.

### 7.2 Attach (browser)
Browser opens `WS /v1/sandboxes/{id}/terminal`. Gateway validates the scoped token, looks up
`provider_ref`, proxies the websocket to the pod's terminal bridge.

### 7.3 Attach (CLI tunnel)
`sandlock attach <id>` opens a local port (e.g. `localhost:5173`), then tunnels it through the
published domain → gateway → pod's supervisor. The CLI never touches Kubernetes; it asks the
control plane to broker the tunnel. This is a smarter, authenticated `kubectl port-forward`.

### 7.4 Teardown
On explicit stop, TTL expiry, or idle timeout, the operator destroys the pod (single-use). The
sandbox row goes to `gone`. The key dies with the pod's memory.

### 7.5 Secret handling (two tiers — do not conflate)
- **Platform secrets** (registry pull creds, supervisor's control-channel auth credential, TLS):
  ordinary Kubernetes Secrets, mounted at pod creation, reconciled by the operator as part of
  desired state.
- **BYOK key**: decrypted in control-plane memory → pushed over the control channel at claim →
  env var on the harness child process → gone on pod destroy. Never a Kubernetes Secret, never
  in etcd, never in the reconciliation loop.

---

## 8. Warm pool & reconciliation

The operator runs a reconciliation loop: compare desired state (`SandboxPool.targetReady`,
each `Sandbox` spec) against actual state (real pods) and fix drift — recreate dead pods, warm
new ones until `targetReady` is met, never exceed `maxTotal`.

Requirements:
- Pods are **single-use**: claimed → used → destroyed → replaced. Never reassign a pod to a
  second user.
- Ready pods get an **idle TTL**; recycle stale warm pods periodically for freshness/security.
- Claiming MUST be atomic (no double-claim).
- The pool is an optimization, not a dependency: empty pool → cold provision on demand.

---

## 9. Isolation & network security

- Run sandbox pods under a hardened **RuntimeClass** — gVisor (`runsc`) is the recommended
  default for syscall-level isolation of agent-executed code. Kata/Firecracker is a later
  premium tier.
- Per-pod CPU/memory limits and ephemeral storage limits (from `Sandbox.spec.resources`).
- **NetworkPolicy**: default-deny. The supervisor's control channel is reachable only from the
  control plane. Egress from the pod is allowlisted (must reach the Anthropic API; everything
  else denied by default) — this is also a selling point: the agent cannot exfiltrate the repo.
- Sandbox pods run in a dedicated namespace; the control plane's service account is RBAC-scoped
  to that namespace only.

---

## 10. Recommended stack & repo layout

```
/cmd/controlplane     Go API + pool manager + gateway (MVP single binary ok)
/cmd/operator         Go controller-runtime operator + CRDs
/cmd/supervisor       Go pod-side binary
/cmd/sandlock         Go CLI
/web                  React + TypeScript dashboard
/deploy               Helm chart / manifests, RuntimeClass, NetworkPolicies, RBAC
/images               base images per harness (claude-code first)
LICENSE               BUSL 1.1 (set Change Date, Additional Use Grant, Change License=Apache-2.0)
```

---

## 11. Build order & milestones

1. **M1 — single cold sandbox, no pool.** Operator + `Sandbox` CRD; CLI `create` cold-provisions
   one pod; supervisor launches Claude Code with a key passed at create; browser terminal via
   gateway. Proves the end-to-end path.
2. **M2 — auth.** Username + password registration and login, password hashing (argon2id),
   session tokens, `sandlock login` using the same credentials, per-user ownership.
3. **M3 — BYOK in-memory injection.** Move the key to the control-channel claim path; verify it
   never lands on disk/etcd/logs. Add encrypted-at-rest storage with opt-out.
4. **M4 — warm pool.** Pool manager, atomic claim, single-use teardown, idle TTL, cold fallback.
5. **M5 — CLI tunnel + dashboard.** Local-port tunnel; dashboard list/open/stop.
6. **M6 — isolation & egress hardening.** gVisor RuntimeClass, default-deny NetworkPolicies,
   egress allowlist, resource limits.

---

## 12. Acceptance criteria

- A user with no kubeconfig can `sandlock login`, `sandlock create`, and reach a working
  Claude Code session in a pod, using their own Anthropic key.
- The BYOK key appears in **no** Kubernetes Secret, **no** etcd entry, **no** log line, and
  **no** process argv — only in the harness process environment, and is gone after teardown.
- Claiming from a warm pool is materially faster than cold provisioning, and an empty pool
  still succeeds via cold fallback.
- A pod is never reused across two sessions.
- Adding a hypothetical second provider requires implementing only the four `Provider` methods
  plus a base image with the supervisor — no changes to gateway, CLI, or pool manager.
- A user cannot list, attach to, or stop another user's sandbox.

---

## 13. Open questions to confirm before/while building
- Exact BUSL parameters: Additional Use Grant wording, Change Date (suggest 4 years),
  Change License (suggest Apache-2.0).
- Whether to store the BYOK key at all in M3, or require it per session (lowest liability).
- Terminal bridge implementation: reuse an existing PTY-over-websocket library vs. write a
  minimal one in the supervisor.
- KMS choice for envelope encryption of stored keys.