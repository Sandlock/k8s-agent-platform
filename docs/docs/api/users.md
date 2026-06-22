---
id: users
title: Users API (Admin)
sidebar_position: 5
---

# Users (Admin)

These endpoints require a valid session token for an account with `is_admin = true`. Non-admin requests return `403 Forbidden`.

---

## `GET /v1/users`

List all users.

**Auth required:** Yes + Admin

### Response `200 OK`

```json
[
  {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "username": "alice",
    "isAdmin": false,
    "createdAt": "2026-06-22T10:00:00Z"
  },
  {
    "id": "b2c3d4e5-f6a7-8901-bcde-fa2345678901",
    "username": "admin",
    "isAdmin": true,
    "createdAt": "2026-06-01T00:00:00Z"
  }
]
```

---

## `POST /v1/users`

Create a new user.

**Auth required:** Yes + Admin

### Request

```json
{
  "username": "bob",
  "password": "initial-password",
  "isAdmin": false
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `username` | string | Yes | Must be unique. |
| `password` | string | Yes | Initial password. The account will have `mustChangePassword = true`. |
| `isAdmin` | bool | No | Grant admin privileges. Default: `false`. |

### Response `201 Created`

```json
{
  "userId": "c3d4e5f6-a7b8-9012-cdef-ab3456789012"
}
```

### Errors

| Status | Condition |
|---|---|
| `409` | Username already taken |
| `400` | Missing required fields |

---

## `DELETE /v1/users/{id}`

Delete a user.

**Auth required:** Yes + Admin

### Response `204 No Content`

### Errors

| Status | Condition |
|---|---|
| `404` | User not found |
| `403` | Attempting to delete your own account |

Deleting a user cascades to:
- All their sessions (immediately invalidated)
- Their stored API key
- Their sandboxes (status set to `gone`)
- Their session snapshots

---

## Admin bootstrap

On first startup, the control plane seeds an `admin` user if one doesn't exist. The password is read from the `ADMIN_PASSWORD` environment variable (set by the Helm chart from the `sandlock-admin` Secret).

The admin account has `must_change_password = true` — you will be prompted to set a permanent password on first login.
