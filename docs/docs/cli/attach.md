---
id: attach
title: sandlock attach
sidebar_position: 3
---

# `sandlock attach`

Attach a terminal to a running sandbox pod.

```
sandlock attach <sandbox-id>
```

## Arguments

| Argument | Description |
|---|---|
| `sandbox-id` | The UUID of the sandbox to attach to. Get this from `sandlock list` or from the output of `sandlock create --detach`. |

## What it does

1. Converts the sandbox's HTTP URL to a WebSocket URL (`ws://` or `wss://`)
2. Dials `wss://<host>/v1/sandboxes/<id>/tunnel?token=<bearer>`
3. Sets the local terminal to raw mode
4. Sends the current terminal size as a resize message
5. Bridges stdin/stdout between your terminal and the PTY inside the pod:
   - **Binary frames**: PTY output → your stdout
   - **Binary frames**: your keystrokes → PTY input
   - **Text frames**: terminal resize JSON → PTY resize

Any prior output (scrollback buffer, up to 256 KB) is replayed immediately on connect so you don't miss output that occurred before you attached.

## Keybindings

| Key | Action |
|---|---|
| **Ctrl+B D** | Detach from the session without terminating it. The pod continues running. |
| **Ctrl+C** | Sends SIGINT to the foreground process inside the pod (normal terminal behavior). |
| **Ctrl+D** | Send EOF to the PTY (exits Claude Code if at the prompt). |

## Signal handling

| Signal | Behavior |
|---|---|
| `SIGWINCH` | Reads the new terminal dimensions and sends a resize message to the pod |
| `SIGINT` | Restores the local terminal and exits `sandlock attach` |
| `SIGTERM` | Restores the local terminal and exits `sandlock attach` |

## Reconnecting

If the WebSocket connection drops (network hiccup, idle timeout), run `sandlock attach <id>` again. The supervisor keeps the PTY alive and replays the scrollback on reconnect.

## Examples

```bash
# Attach to a specific sandbox
sandlock attach 7f3a1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c

# List sandboxes then attach
sandlock list
sandlock attach <id>
```

## Notes

- You can have multiple `sandlock attach` sessions connected to the same sandbox simultaneously. All sessions see the same PTY output.
- The `--server` flag or `SANDLOCK_SERVER` env var controls which control plane is used; the attach URL comes from the server, so no separate configuration is needed.
