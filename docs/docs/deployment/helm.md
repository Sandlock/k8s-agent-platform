---
id: helm
title: Helm Deployment
sidebar_position: 1
---

# Helm Deployment

The Sandlock Helm chart deploys all required resources into your cluster: the control plane, PostgreSQL (optional), namespaces, RBAC, the `SandboxTemplate`, and the `SandboxWarmPool`.

## Prerequisites

- agent-sandbox controller installed ([see Installation](../getting-started/installation.md#1-install-agent-sandbox))
- Helm v3.12+
- A Kubernetes cluster with an ingress controller (nginx, Traefik, etc.)

## Quick install

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set controlplane.ingress.host=sandlock.example.com
```

## What the chart creates

### Namespaces

| Namespace | Purpose |
|---|---|
| `sandlock-system` | Control plane, PostgreSQL, Secrets |
| `sandboxes` | Sandbox pods, Services, `SandboxTemplate`, `SandboxWarmPool` |

### Secrets

| Secret | Namespace | Contents |
|---|---|---|
| `sandlock-admin` | `sandlock-system` | `password` — initial admin password (auto-generated if not set) |
| `sandlock-master-key` | `sandlock-system` | `MASTER_KEY` — 32-byte hex AES key (auto-generated if not set) |
| `sandlock-db` | `sandlock-system` | `DATABASE_URL` — Postgres DSN (bundled or external) |

### Control plane Deployment

- Image: `ghcr.io/sandlock/sandlock-controlplane:latest`
- Replicas: 1
- Port: `8090`
- Mounts all three secrets as environment variables

### PostgreSQL StatefulSet (if `postgresql.enabled = true`)

- Image: `postgres:17`
- Database: `sandlock`
- User: `sandlock`
- Persistent volume: `8Gi` (configurable)

### RBAC

A `ServiceAccount` + `ClusterRole` + `ClusterRoleBinding` granting the control plane permission to:
- Read/write `SandboxClaim` resources in the `sandboxes` namespace
- Read `Sandbox` and `SandboxTemplate` resources

### `SandboxTemplate` (`claude-code`)

Defines the pod spec for sandbox pods:
- Container: supervisor binary (`ghcr.io/sandlock/sandlock-supervisor:latest`)
- Ports: `8080` (control), `8081` (PTY)
- `envVarsInjectionPolicy: Disallowed`
- NetworkPolicy as described in [Security: NetworkPolicy](../architecture/security.md#networkpolicy-isolation)

### `SandboxWarmPool`

Keeps `pool.targetReady` pods pre-warmed. The pool is created in the `sandboxes` namespace.

### Ingress

An `Ingress` resource routes HTTPS traffic to the control plane Service. TLS is configured via cert-manager annotations by default.

## Common install patterns

### External database

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set postgresql.enabled=false \
  --set database.url="postgres://sandlock:pass@my-pg:5432/sandlock?sslmode=require" \
  --set controlplane.ingress.host=sandlock.example.com
```

### Bring your own master key

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set masterKey="$(cat master.key)" \
  --set controlplane.ingress.host=sandlock.example.com
```

### Set a custom admin password

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set admin.password="my-secure-password" \
  --set controlplane.ingress.host=sandlock.example.com
```

### Larger warm pool

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set pool.targetReady=5 \
  --set pool.maxTotal=50 \
  --set controlplane.ingress.host=sandlock.example.com
```

### gVisor sandbox isolation

```bash
helm upgrade --install sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system --create-namespace \
  --set runtimeClass.enabled=true \
  --set controlplane.ingress.host=sandlock.example.com
```

Requires gVisor (`runsc`) installed on your nodes and a `RuntimeClass` named `gvisor`. The chart can create the `RuntimeClass` for you when `runtimeClass.enabled = true`.

## Uninstall

```bash
helm uninstall sandlock --namespace sandlock-system

# Remove namespaces (WARNING: destroys all sandbox pods)
kubectl delete namespace sandlock-system sandboxes
```

:::caution
Deleting the `sandlock-system` namespace destroys the PostgreSQL PVC and all stored data including encrypted API keys and session snapshots.
:::

## Upgrading

```bash
helm upgrade sandlock \
  oci://ghcr.io/sandlock/charts/sandlock \
  --namespace sandlock-system \
  --reuse-values
```

The control plane runs database migrations automatically on startup. Rolling upgrades are safe as long as you run only one replica (the default).
