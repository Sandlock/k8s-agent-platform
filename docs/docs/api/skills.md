---
id: skills
title: Skills API
sidebar_position: 5
---

# Skills API

Skills are user-defined Claude Code custom commands stored server-side and injected into every sandbox at claim time. See the [CLI reference](../cli/skills) for the user-facing workflow.

## `GET /v1/skills`

List all skills for the authenticated user.

**Auth required:** Yes

### Response `200 OK`

```json
[
  {
    "id": "a1b2c3d4-...",
    "name": "code-review",
    "content": "Review the staged diff...",
    "createdAt": "2026-06-20T14:32:11Z"
  }
]
```

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Unique identifier for the skill record |
| `name` | string | Skill name (used as the slash command: `/<name>`) |
| `content` | string | Raw Markdown content of the skill file |
| `createdAt` | timestamp | When the skill was first created (not updated on upsert) |

Returns an empty array `[]` if no skills are stored.

---

## `PUT /v1/skills/{name}`

Create or replace a skill.

**Auth required:** Yes

### Path parameters

| Parameter | Description |
|---|---|
| `name` | Skill name. Must match `^[a-z0-9][a-z0-9_-]{0,63}$`. |

### Request

```json
{
  "content": "Review the staged diff for correctness and security issues."
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `content` | string | Yes | Markdown content for the skill file. Must be non-empty. |

### Response `200 OK`

```json
{
  "id": "a1b2c3d4-...",
  "name": "code-review"
}
```

This endpoint upserts: if a skill with `name` already exists for the user, its content is replaced.

### Errors

| Status | Condition |
|---|---|
| `400` | `name` does not match the allowed pattern, or `content` is missing/empty |
| `501` | Server is running without `DATABASE_URL` configured |

---

## `DELETE /v1/skills/{name}`

Delete a skill.

**Auth required:** Yes

### Path parameters

| Parameter | Description |
|---|---|
| `name` | Name of the skill to delete |

### Response `204 No Content`

Always returns `204`, even if the skill did not exist.

---

## How skills are delivered to sandboxes

When `POST /v1/sandboxes` is called, the control plane:

1. Queries `user_skills WHERE user_id = <caller>` to fetch all stored skills
2. Includes the full list in the `skills` field of the supervisor claim request
3. The supervisor writes each skill to `/home/ubuntu/.claude/commands/<name>.md` before starting Claude Code

Skills are written fresh at claim time — deleting or updating a skill takes effect on the next sandbox you create.
