---
id: github
title: sandlock github
sidebar_position: 7
---

# `sandlock github`

GitHub integration commands for storing a token and interactively picking repos and branches.

---

## `sandlock github token`

Save a GitHub personal access token locally.

```
sandlock github token [token]
```

### Arguments

| Argument | Description |
|---|---|
| `token` | Optional. If omitted, the token is read from stdin with hidden input. |

### Required token scopes

| Scope | Purpose |
|---|---|
| `repo` | List repositories (including private ones) |
| `read:org` | List organization repos |

A classic personal access token or a fine-grained token with "Contents: Read" and "Metadata: Read" permissions both work.

### Token storage

The token is saved to `~/.sandlock/config.yaml` under the `github_token` key:

```yaml
github_token: ghp_...
```

The file is created with permissions `0700`. The token is stored in plaintext locally — keep the file secure.

### Token resolution order

When GitHub operations need a token, the CLI checks these sources in order:

1. `github_token` in `~/.sandlock/config.yaml`
2. `GITHUB_TOKEN` environment variable
3. Output of `gh auth token` (fallback to the local `gh` CLI if installed)

### Examples

```bash
# Interactive (hidden input)
sandlock github token

# Inline
sandlock github token ghp_...

# Via environment variable (no command needed)
export GITHUB_TOKEN=ghp_...
sandlock create --select-repo --use-stored-key
```

---

## `sandlock github repos`

Interactively pick a GitHub repository and print its clone URL to stdout.

```
sandlock github repos
```

### Behavior

1. Fetches up to 500 of your repositories from the GitHub API (`GET /user/repos?sort=pushed`), paginating in batches of 100
2. Opens a fullscreen TUI list with fuzzy search
3. Returns the HTTPS clone URL of the selected repo

### TUI controls

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate the list |
| `/` | Enter filter mode (fuzzy search by repo name) |
| `Enter` | Select and print the clone URL |
| `Esc` / `q` / `Ctrl+C` | Cancel |

### Example

```bash
# Print the clone URL of a chosen repo
sandlock github repos

# Use directly with create
sandlock create --repo $(sandlock github repos) --use-stored-key
```

---

## Branch picker (used by `sandlock create --select-repo`)

When you use `sandlock create --select-repo`, after choosing a repo the branch picker opens automatically. It uses the GitHub GraphQL API to fetch branches in a single request.

### Branch ordering

1. **Default branch** (e.g. `main`) — always shown first
2. **All other branches** — sorted by newest commit (most recently pushed first)
3. **`[ + new branch ]`** — at the bottom, lets you type a name for a new branch

### New branch behavior

If you select `[ + new branch ]` and type a name, the branch is created locally inside the pod with `git checkout -b <name>` after the repo is cloned. It is not pushed to GitHub — you push it from inside the session when ready.

### TUI controls

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate branches |
| `/` | Fuzzy filter |
| `Enter` (on branch) | Select branch |
| `Enter` (on `[ + new branch ]`) | Open text input for new branch name |
| `Enter` (in text input) | Confirm new branch name |
| `Esc` / `Ctrl+C` | Cancel |
