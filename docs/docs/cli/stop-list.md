---
id: stop-list
title: stop & list
sidebar_position: 4
---

# `sandlock stop` and `sandlock list`

## `sandlock stop`

Stop and destroy a running sandbox.

```
sandlock stop <sandbox-id>
```

### What it does

1. **Snapshot** — if the sandbox has a `repo_url`, the control plane sends a snapshot request to the supervisor before deletion. The supervisor tars `~/.claude/` and `~/.claude.json`, and the control plane encrypts and stores it for the next session on the same repo+branch.
2. **Mark gone** — the sandbox record in the database is updated to `status = 'gone'`.
3. **Delete claim** — the `SandboxClaim` Kubernetes resource is deleted. The agent-sandbox controller cascades this to pod deletion.
4. **Warm pool** — the pool controller detects the pod is gone and starts a replacement immediately.

### Example

```bash
sandlock stop 7f3a1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c
```

### Notes

- Stopping a sandbox is permanent. The pod is destroyed and cannot be recovered.
- If no snapshot is taken (no repo URL, or snapshot fails), the session state is simply lost. The repo contents inside the pod are gone — only `~/.claude/` is snapshotted.
- You can also stop a sandbox from the web dashboard.

---

## `sandlock list`

List your sandboxes.

```
sandlock list
```

### Output

```
ID                                     STATUS    REPO                          BRANCH   CREATED
7f3a1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c  running   github.com/my-org/my-repo     main     2026-06-22 13:24:05
a1b2c3d4-e5f6-7890-abcd-ef1234567890  running   -                             -        2026-06-22 11:00:00
```

Sandboxes with `status = 'gone'` are not shown. `REPO` and `BRANCH` show `-` for sandboxes created without a repository.

### Columns

| Column | Description |
|---|---|
| `ID` | Sandbox UUID (use with `sandlock attach` or `sandlock stop`) |
| `STATUS` | Current status: `running`, `failed` |
| `REPO` | Git repository URL the sandbox was cloned from, or `-` if none |
| `BRANCH` | Branch checked out inside the pod, or `-` if none |
| `CREATED` | Creation timestamp |
