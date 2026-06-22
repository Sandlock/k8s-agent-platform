---
id: create
title: sandlock create
sidebar_position: 2
---

# `sandlock create`

Claim a new sandbox pod and optionally attach a terminal to it.

```
sandlock create [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--harness` | string | `claude-code` | Agent harness to run inside the pod. Currently only `claude-code` is supported. |
| `--key` | string | — | Anthropic API key to inject inline. Takes priority over all other key sources. |
| `--use-stored-key` | bool | `false` | Use the key previously stored with `sandlock keys store`. |
| `--repo` | string | — | GitHub (or any Git) repo URL to shallow-clone inside the pod (`--depth 1`). |
| `--branch` | string | — | Git branch to check out after clone. If the branch doesn't exist on the remote it is created locally. |
| `--select-repo` | bool | `false` | Open an interactive TUI picker to choose a GitHub repository. Requires a stored GitHub token. After selecting a repo, a branch picker opens automatically. |
| `-d, --detach` | bool | `false` | Create the sandbox but don't attach. Prints the sandbox ID and attach URL, then exits. |
| `--rm` | bool | `false` | Automatically delete the sandbox when the terminal session ends. |
| `--no-resume` | bool | `false` | Ignore any existing session snapshot for this repo+branch and start fresh. |

## API key resolution order

When the control plane receives a create request it resolves the API key in this order:

1. `--key` flag (inline, highest priority)
2. `ANTHROPIC_API_KEY` environment variable (checked by the CLI before sending)
3. `--use-stored-key` flag (decrypts the key stored via `sandlock keys store`)
4. No key (Claude Code will prompt inside the pod)

The key is sent to the supervisor over the in-cluster control channel at claim time. It is never written to disk or stored in a Kubernetes Secret.

## Examples

### Basic session (no repo)

```bash
sandlock create --use-stored-key
```

### Session with a repository

```bash
sandlock create \
  --repo https://github.com/my-org/my-repo \
  --use-stored-key
```

### Session on a specific branch

```bash
sandlock create \
  --repo https://github.com/my-org/my-repo \
  --branch feature/my-feature \
  --use-stored-key
```

The branch is checked out after clone. If `feature/my-feature` doesn't exist on the remote, it is created locally with `git checkout -b`.

### Interactive repo and branch picker

```bash
sandlock create --select-repo --use-stored-key
```

Opens a fuzzy-searchable TUI list of your GitHub repos (sorted by last push). After selecting a repo, a branch picker opens showing the default branch first, then all other branches sorted by newest commit.

### Detach mode

```bash
SANDBOX_ID=$(sandlock create --repo https://github.com/my-org/my-repo --use-stored-key --detach)
echo "Sandbox: $SANDBOX_ID"
# ... do other things ...
sandlock attach $SANDBOX_ID
```

### Ephemeral session (auto-delete on exit)

```bash
sandlock create --use-stored-key --repo https://github.com/my-org/my-repo --rm
```

The sandbox is deleted when you close the terminal or the Claude Code process exits.

### Override key inline

```bash
sandlock create --key sk-ant-... --repo https://github.com/my-org/my-repo
```

Useful for CI/scripts where you don't want to use the stored key.

### Skip session restore

```bash
sandlock create \
  --repo https://github.com/my-org/my-repo \
  --branch main \
  --use-stored-key \
  --no-resume
```

Ignores any snapshot saved from a previous session on `main` and starts Claude Code fresh.

## What happens after create

1. The control plane creates a `SandboxClaim` resource
2. The agent-sandbox controller assigns a pre-warmed pod from the pool (typically < 1 second)
3. The control plane POSTs the claim to the supervisor at `pod-fqdn:8080/claim`
4. The supervisor clones the repo, checks out the branch, restores the snapshot (if any), and starts Claude Code in a PTY
5. Unless `--detach` is set, `sandlock attach` is called automatically with the returned sandbox ID

## Warm pool wait

If no warm pod is available, the control plane polls every 2 seconds for up to 120 seconds waiting for one to become ready. You will see a waiting message in the CLI. If 120 seconds elapses with no pod ready, the command fails — check `pool.targetReady` in your Helm values.
