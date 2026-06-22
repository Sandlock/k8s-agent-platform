---
id: configuration
title: Configuration Reference
sidebar_position: 2
---

# Configuration Reference

## Helm values

All values can be set with `--set key=value` or in a `values.yaml` file passed with `-f values.yaml`.

### Control plane

| Value | Default | Description |
|---|---|---|
| `controlplane.image.repository` | `ghcr.io/sandlock/sandlock-controlplane` | Container image repository |
| `controlplane.image.tag` | `latest` | Image tag |
| `controlplane.image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `controlplane.replicas` | `1` | Number of control plane replicas |
| `controlplane.resources.requests.cpu` | `100m` | CPU request |
| `controlplane.resources.requests.memory` | `128Mi` | Memory request |
| `controlplane.resources.limits.memory` | `512Mi` | Memory limit |
| `controlplane.ingress.enabled` | `true` | Create an Ingress resource |
| `controlplane.ingress.host` | — | **Required.** Public hostname (e.g. `sandlock.example.com`) |
| `controlplane.ingress.className` | `nginx` | Ingress class name |
| `controlplane.ingress.tls.enabled` | `true` | Enable TLS via cert-manager |
| `controlplane.ingress.tls.clusterIssuer` | `letsencrypt-prod` | cert-manager ClusterIssuer name |
| `controlplane.selfUrl` | auto-detected | In-cluster callback URL. Auto-set to `http://sandlock-controlplane.sandlock-system.svc:8090`. Override if using a custom service name. |

### Warm pool

| Value | Default | Description |
|---|---|---|
| `pool.targetReady` | `1` | Number of pre-warmed pods to keep ready at all times |
| `pool.maxTotal` | `20` | Hard cap on total sandbox pods (ready + claimed) |

### Authentication and secrets

| Value | Default | Description |
|---|---|---|
| `admin.password` | auto-generated | Initial admin password. Auto-generated with a random 24-character string if not set. Retrieve with `kubectl get secret sandlock-admin ...` |
| `masterKey` | auto-generated | 32-byte hex AES-256 master key for BYOK encryption. **Must not change after first install** — changing it makes all existing encrypted keys and snapshots unreadable. Generate with `openssl rand -hex 32`. |

### Database

| Value | Default | Description |
|---|---|---|
| `postgresql.enabled` | `true` | Deploy a bundled PostgreSQL StatefulSet |
| `postgresql.storage` | `8Gi` | PVC size for the bundled PostgreSQL |
| `postgresql.image` | `postgres:17` | PostgreSQL image |
| `database.url` | — | External DSN. When set, `postgresql.enabled` is ignored. Format: `postgres://user:pass@host:5432/dbname?sslmode=require` |

### Sandbox pod

| Value | Default | Description |
|---|---|---|
| `sandbox.image.repository` | `ghcr.io/sandlock/sandlock-supervisor` | Supervisor container image |
| `sandbox.image.tag` | `latest` | Image tag |
| `sandbox.namespace` | `sandboxes` | Namespace where sandbox pods run |
| `sandbox.resources.requests.cpu` | `500m` | CPU request per pod |
| `sandbox.resources.requests.memory` | `1Gi` | Memory request per pod |
| `sandbox.resources.limits.memory` | `4Gi` | Memory limit per pod |

### Runtime isolation

| Value | Default | Description |
|---|---|---|
| `runtimeClass.enabled` | `false` | Use gVisor (`runsc`) for sandbox pods |
| `runtimeClass.name` | `gvisor` | `RuntimeClass` name to create/reference |
| `runtimeClass.create` | `true` | Create the `RuntimeClass` resource (requires gVisor installed on nodes) |

---

## Environment variables (control plane)

The control plane reads these environment variables (all injected from Secrets by the Helm chart):

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `MASTER_KEY` | 32-byte hex AES-256 key for BYOK encryption and snapshot encryption |
| `ADMIN_PASSWORD` | Initial admin user password (used only on first startup) |
| `SELF_URL` | In-cluster base URL for snapshot callbacks (e.g. `http://sandlock-controlplane.sandlock-system.svc:8090`) |
| `SANDBOX_NAMESPACE` | Namespace where `SandboxClaims` are created (default: `sandboxes`) |

---

## CLI configuration

The CLI reads `~/.sandlock/config.yaml`:

```yaml
server: https://sandlock.example.com   # set by --server flag
token: <session-token>                 # set by sandlock login
github_token: ghp_...                  # set by sandlock github token
```

The `SANDLOCK_SERVER` environment variable overrides the `server` value in the config file and the `--server` flag overrides both.

---

## Sizing guide

| Team size | `pool.targetReady` | `pool.maxTotal` | Pod memory limit |
|---|---|---|---|
| 1–5 users | 2 | 10 | 2Gi |
| 5–20 users | 5 | 30 | 4Gi |
| 20–50 users | 10 | 60 | 4Gi |
| 50+ users | 20 | 100+ | 4–8Gi |

Each pod runs the supervisor plus Claude Code, which can be memory-intensive for large repos or long sessions. Monitor actual usage with `kubectl top pods -n sandboxes` and adjust accordingly.
