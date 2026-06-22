---
id: installation
title: Installation
sidebar_position: 1
---

# Installation

## Prerequisites

| Requirement | Version |
|---|---|
| Kubernetes | 1.27+ (EKS, GKE, AKS, kind, etc.) |
| kubectl | Configured for your cluster |
| Helm | v3.12+ |
| agent-sandbox controller | Latest release |

## 1. Install agent-sandbox

Sandlock delegates pod lifecycle management to the [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) controller. Install it first:

```bash
VERSION=$(curl -s https://api.github.com/repos/kubernetes-sigs/agent-sandbox/releases/latest \
  | jq -r '.tag_name')

# Core CRDs + controller
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/manifest.yaml

# Extension CRDs: SandboxTemplate, SandboxWarmPool, SandboxClaim
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/extensions.yaml

# Confirm the controller is ready
kubectl -n agent-sandbox-system get pods
```

## 2. Install the Sandlock Helm chart

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set controlplane.ingress.host=sandlock.example.com
```

This creates:
- A bundled PostgreSQL instance (StatefulSet `postgres-0`)
- `sandlock-admin` Secret with a generated password
- `sandlock-master-key` Secret (randomly generated 32-byte AES key)
- A `SandboxTemplate` named `claude-code` in the `sandboxes` namespace
- A `SandboxWarmPool` keeping 1 pod pre-warmed (configurable)
- RBAC and NetworkPolicy resources

### Using an external database

If you already have PostgreSQL running:

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set postgresql.enabled=false \
  --set database.url="postgres://sandlock:password@my-db:5432/sandlock?sslmode=require" \
  --set controlplane.ingress.host=sandlock.example.com
```

### Using a pre-existing master key

If you're restoring from a backup or migrating, supply your existing master key so encrypted keys and snapshots can be decrypted:

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set masterKey="$(cat master.key)" \
  --set controlplane.ingress.host=sandlock.example.com
```

To generate a new master key:

```bash
openssl rand -hex 32
```

## 3. Verify the installation

```bash
# Control plane pod running
kubectl -n sandlock-system get pods

# One warm sandbox pod pre-started
kubectl -n sandboxes get pods

# Ingress has an address
kubectl -n sandlock-system get ingress
```

## 4. Install the CLI

Download the `sandlock` binary from the [releases page](https://github.com/Sandlock/k8s-agent-platform/releases) for your platform:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/Sandlock/k8s-agent-platform/releases/latest/download/sandlock-darwin-arm64 \
  -o /usr/local/bin/sandlock && chmod +x /usr/local/bin/sandlock

# macOS (Intel)
curl -L https://github.com/Sandlock/k8s-agent-platform/releases/latest/download/sandlock-darwin-amd64 \
  -o /usr/local/bin/sandlock && chmod +x /usr/local/bin/sandlock

# Linux (amd64)
curl -L https://github.com/Sandlock/k8s-agent-platform/releases/latest/download/sandlock-linux-amd64 \
  -o /usr/local/bin/sandlock && chmod +x /usr/local/bin/sandlock
```

Verify:

```bash
sandlock --help
```

### Build from source

Requires Go 1.26+.

```bash
git clone https://github.com/Sandlock/k8s-agent-platform.git
cd k8s-agent-platform
go build -o bin/sandlock ./cmd/sandlock
```

## Next steps

Continue to [Quick Start](./quickstart.md) to create your first session.
