---
id: authentication
title: Authentication API
sidebar_position: 2
---

# Authentication

## `POST /v1/auth/login`

Authenticate and obtain a session token.

**Auth required:** No

### Request

```json
{
  "username": "alice",
  "password": "hunter2"
}
```

### Response `200 OK`

```json
{
  "token": "3f8a9b2c1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a",
  "expiresAt": "2026-06-23T13:24:05Z",
  "isAdmin": false,
  "mustChangePassword": false
}
```

| Field | Type | Description |
|---|---|---|
| `token` | string | 64-character hex bearer token. Include as `Authorization: Bearer <token>`. |
| `expiresAt` | RFC3339 | Token expiry (24 hours from login). |
| `isAdmin` | bool | Whether this account has admin privileges. |
| `mustChangePassword` | bool | If `true`, the user must call `PUT /v1/auth/password` before using other endpoints. |

### Errors

| Status | Condition |
|---|---|
| `401` | Invalid username or password |

---

## `POST /v1/auth/logout`

Invalidate the current session.

**Auth required:** Yes

### Response `204 No Content`

The token is deleted from the database and can no longer be used.

---

## `PUT /v1/auth/password`

Change the authenticated user's password.

**Auth required:** Yes

### Request

```json
{
  "currentPassword": "hunter2",
  "newPassword": "correct-horse-battery-staple"
}
```

### Response `204 No Content`

The password hash is updated and `must_change_password` is cleared.

### Errors

| Status | Condition |
|---|---|
| `401` | `currentPassword` is incorrect |
| `400` | Missing required fields |

---

## `POST /v1/auth/register`

Create a new user account.

**Auth required:** No

### Request

```json
{
  "username": "alice",
  "password": "correct-horse-battery-staple"
}
```

### Response `201 Created`

(empty body)

The new account has `must_change_password = true` and `is_admin = false`.

### Errors

| Status | Condition |
|---|---|
| `409` | Username already taken |
| `400` | Missing fields |

---

## Token security details

- Tokens are 32 cryptographically random bytes encoded as 64 hex characters
- The server stores only `SHA-256(token)` — the plaintext is never persisted
- Tokens expire after **24 hours**
- Each login creates a new session; prior sessions remain valid until they expire or are explicitly logged out
- Passwords are hashed with **argon2id** (time=1, memory=64 MB, parallelism=4, key length=32 bytes)
