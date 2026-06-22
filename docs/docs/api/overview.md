---
id: overview
title: API Overview
sidebar_position: 1
---

# API Reference

The Sandlock control plane exposes a REST API on port `8090`. All endpoints use JSON unless noted.

## Base URL

```
https://sandlock.example.com
```

## Authentication

Most endpoints require a bearer token obtained from `POST /v1/auth/login`:

```
Authorization: Bearer <token>
```

The token can also be passed as a query parameter for WebSocket connections:

```
wss://sandlock.example.com/v1/sandboxes/{id}/tunnel?token=<token>
```

See [Authentication](./authentication.md) for login, registration, and password management.

## Endpoint groups

| Group | Base path | Auth required |
|---|---|---|
| [Authentication](./authentication.md) | `/v1/auth` | No (except logout and password change) |
| [Sandboxes](./sandboxes.md) | `/v1/sandboxes` | Yes |
| [API Keys](./keys.md) | `/v1/keys` | Yes |
| [Users (Admin)](./users.md) | `/v1/users` | Yes + Admin |
| Internal (control plane → pod) | `/v1/internal` | In-cluster only |

## Standard error responses

All error responses use this shape:

```json
{
  "error": "human-readable message"
}
```

| Status | Meaning |
|---|---|
| `400 Bad Request` | Invalid request body or missing required fields |
| `401 Unauthorized` | Missing, invalid, or expired token |
| `403 Forbidden` | Valid token but insufficient permissions (e.g., not admin) |
| `404 Not Found` | Resource not found or not owned by you |
| `409 Conflict` | Username already taken, or sandbox already claimed |
| `500 Internal Server Error` | Unexpected server error |

## Health endpoints

### `GET /healthz`

Kubernetes liveness probe.

**Response:** `200 OK` (empty body)

### `GET /readyz`

Kubernetes readiness probe.

**Response:** `200 OK` (empty body)

## Web dashboard

`GET /*` serves the embedded React web dashboard. Unknown paths fall back to `index.html` for SPA routing.
