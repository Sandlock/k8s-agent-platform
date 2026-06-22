# Sandlock

Sandlock runs [Claude Code](https://claude.ai/code) agents in isolated, ephemeral Kubernetes pods. Each session gets its own pod, its own network policy, and its own copy of your repository — torn down automatically when the session ends.

It is built on top of [kubernetes-sigs/agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox), which handles the low-level pod lifecycle: warm pool pre-warming, atomic claim/adopt, headless Service creation, and NetworkPolicy enforcement. Sandlock adds the application layer on top:

- **Control plane** — REST API with session auth, per-user BYOK key storage (AES-256-GCM), sandbox lifecycle management, and a WebSocket terminal proxy
- **Supervisor** — a small binary that runs inside every pod, provides a control channel (`:8080`) for receiving the repo + API key at claim time, and a PTY bridge (`:8081`) for the terminal
- **CLI** — `sandlock create / attach / stop / list` — users need no kubeconfig
- **Web dashboard** — React + xterm.js UI: browser terminal, sandbox list, user management

## Architecture

```
                        ┌─────────────────────────────────────┐
                        │           Kubernetes cluster         │
  Browser / CLI         │                                      │
  ─────────────  ──────►│  sandlock-controlplane (API + proxy) │
                        │        │          │                  │
                        │        │ SandboxClaim                │
                        │        ▼          │                  │
                        │  agent-sandbox    │                  │
                        │  controller ──────► warm pod pool    │
                        │                   │  (supervisors)   │
                        │                   │                  │
                        │              claim adopted ──► pod   │
                        │                        :8080 control │
                        │                        :8081 PTY     │
                        └─────────────────────────────────────┘
```

Pods are single-use. The supervisor refuses a second claim. When the session ends the SandboxClaim is deleted and agent-sandbox destroys the pod; the warm pool immediately starts a replacement.

## Quickstart

### Prerequisites

- Kubernetes cluster (EKS, GKE, AKS, kind, etc.)
- `kubectl` configured for the cluster
- `helm` v3
- `sandlock` CLI ([releases](https://github.com/sandlock/k8s-agent-platform/releases))

### 1. Install agent-sandbox

Sandlock requires the agent-sandbox controller and CRDs:

```bash
VERSION=$(curl https://api.github.com/repos/kubernetes-sigs/agent-sandbox/releases/latest | jq -r '.tag_name')

# Core components (CRDs + controller):
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/manifest.yaml

# Extensions components (SandboxTemplate, SandboxWarmPool, SandboxClaim):
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/extensions.yaml

# Wait for the controller to be ready:
kubectl -n agent-sandbox-system get pods
```

### 2. Install Sandlock

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set controlplane.ingress.host=sandlock.example.com
```

This creates:
- A bundled PostgreSQL instance for auth + key storage
- A `sandlock-admin` Secret with a generated password
- A `SandboxTemplate` and `SandboxWarmPool` (1 ready pod by default)

### 3. Get the admin password

```bash
kubectl get secret sandlock-admin -n sandlock-system \
  -o jsonpath='{.data.password}' | base64 -d && echo
```

Open `https://sandlock.example.com`, log in as `admin` with that password, and you will be prompted to set a permanent password.

### 4. Create a session

```bash
sandlock login --server https://sandlock.example.com
sandlock keys store          # paste your Anthropic API key — encrypted at rest
sandlock create --repo https://github.com/your-org/your-repo
```

A warm pod is claimed in under a second. Your repo is cloned inside it and Claude Code starts automatically.

## Helm values

| Key | Default | Description |
|---|---|---|
| `controlplane.ingress.host` | — | Public hostname for the control plane |
| `pool.targetReady` | `1` | Number of warm pods to keep ready |
| `pool.maxTotal` | `20` | Hard cap on total sandbox pods |
| `admin.password` | auto-generated | Override the initial admin password |
| `postgresql.enabled` | `true` | Use bundled PostgreSQL |
| `database.url` | — | External DSN (disables bundled PostgreSQL) |
| `masterKey` | — | 32-byte hex key for BYOK encryption (`openssl rand -hex 32`) |

## Repository layout

```
cmd/controlplane   API server, WebSocket proxy, admin seed
cmd/supervisor     Pod-side binary — control channel + PTY bridge
cmd/sandlock       CLI (create, attach, list, stop, login, keys)
internal/api       HTTP handlers, middleware, auth
internal/auth      argon2id passwords, session tokens, AES-256-GCM BYOK
internal/db        Migrations (golang-migrate), pgx pool
deploy/chart       Helm chart (includes SandboxTemplate + SandboxWarmPool)
images/claude-code Supervisor container image
web/               React 19 + TypeScript dashboard (xterm.js terminal)
```

## Security notes

- BYOK keys are injected into the pod only at claim time over the in-cluster control channel — never stored in a Kubernetes Secret or environment variable at rest
- `SandboxTemplate` sets `envVarsInjectionPolicy: Disallowed` — no env vars can leak into pods via SandboxClaim
- NetworkPolicy allows inbound traffic only from `sandlock-system` on the control and terminal ports; egress is limited to HTTPS + DNS
- Passwords are hashed with argon2id; session tokens are stored as SHA-256 hashes
