---
id: keys
title: sandlock keys
sidebar_position: 6
---

# `sandlock keys`

Manage your encrypted Anthropic API key stored on the control plane.

## Why store a key?

Storing your key lets you run `sandlock create --use-stored-key` without passing the key on the command line or setting an environment variable. The key is encrypted with AES-256-GCM under a master key controlled by the Sandlock operator — it is never stored in plaintext.

Each user can have at most one stored key. Storing a new key replaces the existing one.

---

## `sandlock keys store`

Encrypt and store an Anthropic API key.

```
sandlock keys store [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--key` | API key to store. If omitted, the CLI reads from `ANTHROPIC_API_KEY` env var, or prompts securely. |

### Examples

```bash
# Interactive (recommended — input is hidden)
sandlock keys store

# From environment variable
ANTHROPIC_API_KEY=sk-ant-... sandlock keys store

# Inline (visible in shell history)
sandlock keys store --key sk-ant-...
```

### Response

```
Key stored. Hint: ...wxyz
```

The hint is the last 4 characters of your API key. It lets you confirm which key is stored without revealing it.

---

## `sandlock keys delete`

Delete the stored API key.

```
sandlock keys delete
```

After deletion, `sandlock create --use-stored-key` will fail until you store a new key.

### Example

```bash
sandlock keys delete
# Stored key deleted.
```

---

## How encryption works

1. When you run `sandlock keys store`, the CLI sends the plaintext key to the control plane over HTTPS
2. The control plane encrypts it with **AES-256-GCM** using the `MASTER_KEY` (a 32-byte hex key held in a Kubernetes Secret)
3. Only the ciphertext (`nonce || ciphertext`) is written to the database — the plaintext is immediately discarded
4. When `sandlock create --use-stored-key` is used, the control plane decrypts the key in-memory and POSTs it to the supervisor over the in-cluster control channel
5. The supervisor sets it as `ANTHROPIC_API_KEY` in the Claude Code process environment — it is never written to disk inside the pod

See [Security Model](../architecture/security.md#byok-key-encryption) for the full technical details.
