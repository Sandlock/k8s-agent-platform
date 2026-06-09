# Sandlock

Run Claude Code agents in isolated, ephemeral Kubernetes pods.
Users interact through the CLI and web dashboard — no kubeconfig required.

## Quickstart (local kind cluster)

**Prerequisites:** Go 1.26, Docker/Podman, kind, kubectl

```bash
# 1. Spin up a local cluster
make kind-up

# 2. Apply CRDs
make deploy-crds

# 3. Build + load images
make docker-build kind-load

# 4. Deploy operator + control plane
make deploy

# 5. Forward the API locally
kubectl port-forward -n sandlock-system svc/sandlock-controlplane 8090:8090 &

# 6. Create a sandbox (M1 dev mode — no auth/DB needed)
./bin/sandlock create --key $ANTHROPIC_API_KEY

# 7. Attach (opens tunnel + browser)
./bin/sandlock attach <sandbox-id>
```

## With auth + BYOK key storage (M2/M3)

```bash
docker run -d --name sandlock-pg -e POSTGRES_PASSWORD=sandlock \
  -p 5432:5432 postgres:17

export DATABASE_URL="postgres://postgres:sandlock@localhost:5432/postgres?sslmode=disable"
export MASTER_KEY="$(openssl rand -hex 32)"

./bin/sandlock login
./bin/sandlock create --use-stored-key
```

## Web dashboard (M5)

```bash
cd web && npm install && npm run dev   # http://localhost:5173
```

## Milestones

| | Milestone |
|---|---|
| ✅ | M1 — single cold sandbox, browser terminal |
| ✅ | M2 — auth (register / login / session tokens) |
| ✅ | M3 — BYOK key encrypted at rest (AES-256-GCM) |
| ✅ | M4 — warm pool, atomic claim, cold fallback |
| ✅ | M5 — CLI tunnel, React + xterm.js dashboard |
| ✅ | M6 — gVisor RuntimeClass, NetworkPolicies, resource limits |

## Repository layout

```
cmd/controlplane   API server + pool manager + WS gateway
cmd/operator       Kubernetes CRD reconciler
cmd/supervisor     Pod-side binary (control channel + PTY bridge)
cmd/sandlock       CLI (create, attach, list, stop, login, logout)
internal/          auth, api, db, pool, provider, supervisorproto
api/v1alpha1/      Sandbox + SandboxPool CRD Go types
config/crd/        CRD YAML manifests
deploy/            K8s manifests, RBAC, NetworkPolicies, RuntimeClass
images/            Base container image (claude-code + supervisor)
web/               React 19 + TypeScript dashboard (xterm.js terminal)
```
