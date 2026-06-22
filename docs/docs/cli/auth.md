---
id: auth
title: login, logout & register
sidebar_position: 5
---

# Authentication commands

## `sandlock login`

Log in to the control plane and save a session token.

```
sandlock login [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--username` | Username (prompts if omitted) |
| `--password` | Password (prompts securely with hidden input if omitted) |

### Behavior

1. POSTs credentials to `POST /v1/auth/login`
2. Receives a session token (valid for 24 hours)
3. Saves the token to `~/.sandlock/config.yaml`
4. If `mustChangePassword` is true on the account, you will be prompted to set a new password immediately

### Examples

```bash
# Interactive (recommended)
sandlock login --server https://sandlock.example.com

# Non-interactive (CI/scripts)
sandlock login \
  --server https://sandlock.example.com \
  --username myuser \
  --password "$MY_PASSWORD"
```

:::caution
Passing `--password` on the command line exposes it in shell history and `ps` output. Use the interactive prompt or an environment variable in scripts.
:::

---

## `sandlock logout`

Invalidate the current session and remove the token from local config.

```
sandlock logout
```

### Behavior

1. Calls `POST /v1/auth/logout` with the current bearer token
2. Removes the `token` key from `~/.sandlock/config.yaml`

After logout, all commands that require authentication will fail until you run `sandlock login` again.

---

## `sandlock register`

Create a new account.

```
sandlock register [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--username` | Username (prompts if omitted) |

### Behavior

1. Prompts for username (if not passed as a flag)
2. Prompts for a password (hidden input, confirmed)
3. POSTs to `POST /v1/auth/register`
4. The new account has `must_change_password = true`

:::note
Self-registration must be enabled by your Sandlock administrator. In many deployments, user accounts are created by an admin via the web dashboard or `POST /v1/users`.
:::

### Example

```bash
sandlock register --server https://sandlock.example.com
# Username: alice
# Password: (hidden)
# Confirm: (hidden)
# Registered successfully.
```

---

## Session token storage

Session tokens are stored in `~/.sandlock/config.yaml`:

```yaml
server: https://sandlock.example.com
token: 3f8a9b2c1d4e5f6a7b8c9d0e1f2a3b4c...
```

The file is created with permissions `0700`. The token is a 64-character hex string; on the server it is stored as a SHA-256 hash, so the plaintext token is never persisted server-side.

Tokens expire after **24 hours**. After expiry, commands will return a `401 Unauthorized` error — run `sandlock login` to get a new one.
