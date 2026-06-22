---
id: keys
title: Keys API
sidebar_position: 4
---

# API Keys

Each user can store one Anthropic API key. The key is encrypted at rest with AES-256-GCM and decrypted only in-memory at sandbox claim time.

---

## `POST /v1/keys`

Store (or replace) the authenticated user's Anthropic API key.

**Auth required:** Yes

### Request

```json
{
  "anthropicKey": "sk-ant-api03-..."
}
```

### Response `201 Created`

```json
{
  "hint": "...abcd"
}
```

`hint` is the last 4 characters of the key — useful for confirming which key is stored without revealing it.

If a key was already stored, it is replaced.

### Errors

| Status | Condition |
|---|---|
| `400` | `anthropicKey` field missing or empty |

---

## `DELETE /v1/keys`

Delete the stored API key.

**Auth required:** Yes

### Response `204 No Content`

After deletion, `POST /v1/sandboxes` with `useStoredKey: true` will return `400`.

---

## Encryption details

See [Security Model: BYOK Key Encryption](../architecture/security.md#byok-key-encryption) for the full technical explanation of how keys are encrypted and decrypted.
