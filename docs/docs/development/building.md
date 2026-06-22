---
id: building
title: Building from Source
sidebar_position: 1
---

# Building from Source

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.26+ | Build all binaries |
| Node.js | 18+ | Build the web dashboard |
| Docker | 24+ | Build container images |
| kind | Latest | Local Kubernetes cluster |
| helm | v3.12+ | Deploy to kind |
| kubectl | 1.27+ | Interact with the cluster |

## Repository layout

```
cmd/
  controlplane/    Control plane HTTP server
  supervisor/      Pod-side supervisor binary
  sandlock/        CLI
internal/
  api/             HTTP handlers and middleware
  auth/            argon2id, AES-256-GCM, session tokens
  db/              PostgreSQL pool and migrations
  supervisorproto/ ClaimRequest / ClaimResponse types
web/               React 19 + TypeScript dashboard (xterm.js)
deploy/chart/      Helm chart
images/            Supervisor container Dockerfile
build/             Control plane / operator Dockerfiles
```

## Building the CLI

```bash
go build -o bin/sandlock ./cmd/sandlock
```

### Cross-compile for macOS

```bash
make build-sandlock-darwin
# Produces:
#   bin/sandlock-darwin-amd64
#   bin/sandlock-darwin-arm64
```

## Building all binaries

```bash
make build
# Produces:
#   bin/operator
#   bin/supervisor
#   bin/controlplane
#   bin/sandlock
```

## Running tests

```bash
make test
# or
go test ./... -v -count=1
```

## Building the web dashboard

```bash
make web-build
# or
cd web && npm install && npm run build
```

The dashboard is compiled to `web/dist/` and embedded into the control plane binary at compile time via Go's `embed` package.

For development with hot reload:

```bash
make web-dev
# or
cd web && npm run dev
```

## Building container images

```bash
make docker-build
```

This builds three images:
- `sandlock/supervisor:latest` — supervisor binary (from `images/claude-code/Dockerfile`)
- `sandlock/operator:latest` — operator binary (from `build/Dockerfile.operator`)
- `sandlock/controlplane:latest` — control plane binary (from `build/Dockerfile.operator`)

## Local development with kind

### Start a cluster

```bash
make kind-up
# Creates a kind cluster named "sandlock" and sets your kubecontext
```

### Install agent-sandbox

Follow the [installation guide](../getting-started/installation.md#1-install-agent-sandbox) to install the agent-sandbox controller into the kind cluster.

### Load images into kind

```bash
make kind-load
# Loads all three built images into the kind cluster
```

### Deploy Sandlock

```bash
helm upgrade --install sandlock deploy/chart \
  --namespace sandlock-system --create-namespace \
  --set controlplane.ingress.enabled=false
```

For development without an ingress controller, port-forward instead:

```bash
kubectl -n sandlock-system port-forward svc/sandlock-controlplane 8090:8090
```

Then use:

```bash
./bin/sandlock --server http://localhost:8090 login
```

### Tear down

```bash
make kind-down
```

## CI / GitHub Actions

The repository includes two workflows:

### `.github/workflows/build-containers.yaml`

Builds and pushes the control plane and supervisor images to `ghcr.io/sandlock/` on push to `main`. Only runs when files relevant to each image have changed (path filters on `cmd/`, `internal/`, `web/`, `images/`, `build/`).

### `.github/workflows/helm-lint.yaml`

Runs `helm lint deploy/chart` on pull requests that touch `deploy/chart/`.

## Adding a migration

1. Create two files in `internal/db/migrations/`:

```
000005_my_change.up.sql
000005_my_change.down.sql
```

2. Write the up migration. Keep it backward compatible if possible.
3. The down migration should exactly reverse the up migration.
4. Migrations run automatically on next control plane startup.

Migrations are embedded into the binary with `//go:embed migrations/*.sql`, so no separate file distribution is needed.
