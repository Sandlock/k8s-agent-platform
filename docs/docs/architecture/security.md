---
id: security
title: Security Model
sidebar_position: 3
---

# Security Model

## Threat model summary

| Threat | Mitigation |
|---|---|
| API key leaked at rest | AES-256-GCM encryption; ciphertext only in DB |
| API key leaked in Kubernetes | Key injected over in-cluster control channel, never in a Secret or env var at rest |
| Session token stolen | SHA-256 hashed at rest; 24-hour expiry |
| Password brute force | argon2id (memory-hard); no lockout needed for remote attackers |
| Pod-to-pod lateral movement | NetworkPolicy default-deny; pods can only talk to control plane |
| Pod reaching internal infra | Egress limited to HTTPS (443) and DNS (53) |
| Second user claiming same pod | Supervisor atomically rejects second claim |
| Snapshot data leaked | Snapshots encrypted with AES-256-GCM at same key as BYOK keys |

---

## Password hashing

Passwords are hashed with **argon2id** before storage. The parameters are:

| Parameter | Value |
|---|---|
| Time cost | 1 |
| Memory | 64 MB |
| Parallelism | 4 |
| Output length | 32 bytes |
| Salt length | 16 bytes (random per hash) |

**Storage format:** `hex(salt) + "$" + hex(hash)` — a 97-character string.

Verification uses constant-time comparison to prevent timing attacks.

---

## Session tokens

1. On login, the server generates **32 cryptographically random bytes** and encodes them as a 64-character hex string
2. The raw token is returned to the client once and never stored
3. The server stores **`SHA-256(raw_token)`** in the `sessions` table
4. On each request, the server computes `SHA-256(submitted_token)` and looks it up — the plaintext is never held server-side
5. Sessions expire after **24 hours** (`expires_at` checked on every request)

---

## BYOK key encryption

Anthropic API keys are encrypted with **AES-256-GCM** (authenticated encryption) before database storage.

### Key derivation

The `MASTER_KEY` is a 32-byte hex string injected at startup via environment variable (sourced from the `sandlock-master-key` Kubernetes Secret). It is parsed once at `init()` time; the process panics if it is missing or malformed.

The master key is generated during Helm install with `openssl rand -hex 32` if not supplied.

### Encryption (`EncryptKey`)

```
1. Generate 12-byte random nonce
2. AES-256-GCM(key=MASTER_KEY, nonce=nonce, plaintext=apiKey)
3. Store: nonce || ciphertext (binary, concatenated)
```

### Decryption (`DecryptKey`)

```
1. Split stored bytes: nonce = first 12 bytes, ciphertext = rest
2. AES-256-GCM.Open(key=MASTER_KEY, nonce=nonce, ciphertext=ciphertext)
3. Return plaintext string
```

The GCM authentication tag is included in the ciphertext and verified on decryption, preventing tampering.

### Key flow at claim time

```
User key (plaintext)
    │
    │  HTTPS to control plane
    ▼
POST /v1/keys  ──► AES-256-GCM encrypt ──► ciphertext in DB

POST /v1/sandboxes (useStoredKey=true)
    │
    │  DB query
    ▼
Decrypt ciphertext (in-memory only)
    │
    │  In-cluster HTTP (NetworkPolicy protected)
    ▼
POST pod:8080/claim  { anthropicKey: "sk-ant-..." }
    │
    │  In-memory only in supervisor
    ▼
claude process env: ANTHROPIC_API_KEY=sk-ant-...
    │
    │  Never written to disk
    ▼
Process exits ──► key is garbage collected
```

---

## Session snapshot encryption

Session snapshots (`~/.claude/` archives) are encrypted with the same AES-256-GCM scheme using the same `MASTER_KEY`. This protects conversation history and Claude Code configuration.

---

## NetworkPolicy isolation

Every sandbox pod runs under a `NetworkPolicy` (enforced by the agent-sandbox controller via `SandboxTemplate`):

```yaml
networkPolicy:
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: sandlock-system
      ports:
        - protocol: TCP
          port: 8080   # supervisor control channel
        - protocol: TCP
          port: 8081   # PTY bridge
  egress:
    - ports:
        - protocol: TCP
          port: 443    # HTTPS (GitHub, Anthropic API, npm, etc.)
        - protocol: UDP
          port: 53     # DNS
```

**Inbound:** Only the `sandlock-system` namespace can reach the pod on ports 8080 and 8081. No external or user-to-pod direct traffic is possible.

**Outbound:** The pod can only make HTTPS requests (to GitHub, the Anthropic API, npm registries, etc.) and DNS queries. It cannot reach other pods, internal cluster services, or cloud metadata endpoints.

---

## `SandboxTemplate` env var injection policy

```yaml
spec:
  envVarsInjectionPolicy: Disallowed
```

This prevents env vars from being injected into the pod via `SandboxClaim`. Without this, a malicious or misconfigured claim could inject `ANTHROPIC_API_KEY` directly into the pod spec (visible in `kubectl describe pod`). Instead, the key is transmitted over the in-cluster control channel and held only in the supervisor's process memory.

---

## Single-use pods

The supervisor uses an `atomic.Bool` to enforce single-use:

```go
// handleClaim rejects any second claim
if m.claimed.Swap(true) {
    http.Error(w, "already claimed", http.StatusConflict)
    return
}
```

`Swap` atomically returns the old value. If it was already `true`, the claim is rejected with `409 Conflict`. This is race-safe even under concurrent requests.

---

## gVisor (optional)

For stricter kernel isolation, Sandlock supports [gVisor](https://gvisor.dev) via a `RuntimeClass`. When `runtimeClass.enabled = true` in Helm values, sandbox pods use the `gvisor` runtime class, which intercepts system calls and runs them in a user-space kernel.

This prevents sandbox processes from exploiting kernel vulnerabilities even if they escape the container.
