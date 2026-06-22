---
id: intro
title: Introduction
sidebar_position: 1
slug: /intro
---

# Sandlock

Sandlock runs [Claude Code](https://claude.ai/code) agents in isolated, ephemeral Kubernetes pods. Every session gets its own pod, its own cloned repository, and its own network policy — torn down automatically when the session ends.

It is built on top of [kubernetes-sigs/agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox), which handles the low-level pod lifecycle: warm pool pre-warming, atomic claim/adopt, headless Service creation, and NetworkPolicy enforcement. Sandlock adds the application layer:

| Component | Role |
|---|---|
| **Control plane** | REST API, session auth, BYOK key storage, WebSocket terminal proxy |
| **Supervisor** | Pod-side binary: control channel (`:8080`) + PTY bridge (`:8081`) |
| **CLI** | `sandlock create / attach / stop / list` — no kubeconfig needed |
| **Web dashboard** | React + xterm.js: browser terminal, sandbox list, user management |

## Key properties

**Single-use pods.** The supervisor atomically rejects a second claim. When a session ends, the `SandboxClaim` is deleted and agent-sandbox destroys the pod; the warm pool immediately starts a replacement.

**BYOK key encryption.** Anthropic API keys are encrypted with AES-256-GCM under a master key you control. The key is decrypted only in-memory at claim time, POSTed to the supervisor over the in-cluster control channel, and never persisted to disk or Kubernetes Secrets.

**Session snapshots.** On exit, the supervisor tars `~/.claude/`, encrypts it, and POSTs it to the control plane. The next `sandlock create` for the same repo+branch automatically decrypts and restores it.

**Network isolation.** Sandbox pods accept inbound traffic only from the `sandlock-system` namespace. Egress is limited to HTTPS (443) and DNS (53). Pods cannot reach each other or internal cluster services.

## Architecture overview

```
                        ┌──────────────────────────────────────────┐
                        │           Kubernetes cluster              │
  Browser / CLI         │                                           │
  ──────────── ────────►│  sandlock-controlplane  (API + proxy)    │
                        │         │           │                     │
                        │         │ SandboxClaim                    │
                        │         ▼           │                     │
                        │   agent-sandbox     │                     │
                        │   controller ───────► warm pod pool       │
                        │                     │  (supervisors)      │
                        │                claim adopted              │
                        │                     ▼                     │
                        │              pod (supervisor)             │
                        │                :8080 control channel      │
                        │                :8081 PTY bridge           │
                        └──────────────────────────────────────────┘
```

## Next steps

- [Installation](./getting-started/installation.md) — install the CLI and deploy the Helm chart
- [Quick Start](./getting-started/quickstart.md) — create your first session in 5 minutes
- [CLI Reference](./cli/overview.md) — all commands and flags
- [Architecture: How It Works](./architecture/overview.md) — deep dive into the full request flow
