---
id: overview
title: CLI Overview
sidebar_position: 1
---

# CLI Reference

The `sandlock` CLI is the primary way users interact with the platform. It requires no Kubernetes access — all operations go through the control plane REST API.

## Global flags

All commands accept these flags:

| Flag | Default | Description |
|---|---|---|
| `--server` | `http://localhost:8090` | Control plane base URL. Can also be set via `SANDLOCK_SERVER` env var or `server` key in `~/.sandlock/config.yaml`. |

## Configuration file

The CLI reads and writes `~/.sandlock/config.yaml`:

```yaml
server: https://sandlock.example.com
token: <session-token>         # set by sandlock login
github_token: ghp_...          # set by sandlock github token
```

The directory is created with mode `0700` on first write.

## Command summary

| Command | Description |
|---|---|
| [`sandlock create`](./create.md) | Claim a new sandbox pod, optionally attach |
| [`sandlock attach <id>`](./attach.md) | Attach terminal to a running sandbox |
| [`sandlock stop <id>`](./stop-list.md) | Stop and destroy a sandbox |
| [`sandlock list`](./stop-list.md) | List your sandboxes |
| [`sandlock login`](./auth.md) | Log in and save session token |
| [`sandlock logout`](./auth.md) | Invalidate session |
| [`sandlock register`](./auth.md) | Create a new account |
| [`sandlock keys store`](./keys.md) | Encrypt and store an Anthropic API key |
| [`sandlock keys delete`](./keys.md) | Delete the stored API key |
| [`sandlock github token`](./github.md) | Save a GitHub personal access token |
| [`sandlock github repos`](./github.md) | Interactively pick a repo (prints clone URL) |

## Authentication flow

1. Run `sandlock login` — your session token is saved to `~/.sandlock/config.yaml`
2. All subsequent commands include `Authorization: Bearer <token>` automatically
3. Sessions expire after 24 hours — run `sandlock login` again to refresh
