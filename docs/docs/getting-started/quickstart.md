---
id: quickstart
title: Quick Start
sidebar_position: 2
---

# Quick Start

This guide takes you from a fresh Sandlock installation to a running Claude Code session in under 5 minutes.

## 1. Get the admin password

After installing the Helm chart, retrieve the auto-generated admin password:

```bash
kubectl get secret sandlock-admin -n sandlock-system \
  -o jsonpath='{.data.password}' | base64 -d && echo
```

## 2. Log in

```bash
sandlock login --server https://sandlock.example.com
# Username: admin
# Password: <paste the password from step 1>
```

Your session token is saved to `~/.sandlock/config.yaml` and will be used automatically by subsequent commands.

On first login you will be prompted to change the admin password. You can also do this at any time:

```bash
sandlock login --server https://sandlock.example.com
# Follow the password change prompt
```

## 3. Store your Anthropic API key

```bash
sandlock keys store
# Paste your Anthropic API key when prompted (input is hidden)
# Key hint: ...abcd
```

The key is encrypted with AES-256-GCM on the server and never stored in plaintext. The "hint" shown (last 4 characters) lets you confirm which key is stored without revealing it.

## 4. Create a session

### With a GitHub repo

```bash
sandlock create --repo https://github.com/your-org/your-repo --use-stored-key
```

### With interactive repo and branch picking

Store a GitHub token first:

```bash
sandlock github token
# Paste your GitHub personal access token (needs repo scope)
```

Then create with interactive pickers:

```bash
sandlock create --select-repo --use-stored-key
# → pick a repo from the list
# → pick a branch (sorted by newest commit, default branch first)
```

### Without a repo

```bash
sandlock create --use-stored-key
```

Claude Code starts in an empty workspace.

## 5. Work in the session

Once created, your terminal is attached to the Claude Code PTY inside the pod. You have full Claude Code functionality:

- The repository is cloned and the branch is checked out
- `ANTHROPIC_API_KEY` is set inside the pod
- If `--use-stored-key` was used and a prior snapshot exists for this repo+branch, `~/.claude/` is restored automatically

### Detach without stopping

Press **Ctrl+B D** to detach from the session without terminating it. The pod keeps running.

### Reattach

```bash
# List your sandboxes
sandlock list

# Reattach to a running one
sandlock attach <sandbox-id>
```

## 6. Stop a session

```bash
sandlock stop <sandbox-id>
```

This:
1. Tells the supervisor to snapshot `~/.claude/` and push it to the control plane
2. Marks the sandbox as `gone` in the database
3. Deletes the `SandboxClaim`, which cascades to pod deletion
4. The warm pool controller starts a replacement pod

## What's next

- [CLI Reference](../cli/overview.md) — all flags and commands
- [GitHub Integration](../cli/github.md) — repo and branch pickers
- [Session Persistence](../architecture/lifecycle.md#session-snapshots) — how snapshots work
- [Deployment Configuration](../deployment/configuration.md) — tune the warm pool and ingress
