---
id: skills
title: sandlock skills
sidebar_position: 7
---

# `sandlock skills`

Manage skills — custom Claude Code slash commands that are automatically injected into every sandbox session you start.

Skills are Markdown files written to `~/.claude/commands/<name>.md` inside the sandbox before Claude Code starts. This makes them available as `/<name>` inside the session, exactly like any other Claude Code custom command.

## Subcommands

### `sandlock skills list`

List all skills stored for your account.

```bash
sandlock skills list
```

Prints each skill's name and the date it was created:

```
  my-review                                 2026-06-20T14:32:11Z
  setup-python                              2026-06-21T09:15:44Z
```

If no skills are stored, prints a help message instead.

---

### `sandlock skills put <name>`

Create or replace a skill. The name must match `^[a-z0-9][a-z0-9_-]{0,63}$` (lowercase alphanumeric, hyphens, underscores; max 64 characters).

```bash
sandlock skills put <name> [--file <path>]
```

| Flag | Description |
|---|---|
| `--file <path>` | Read skill content from a file. If omitted, reads from stdin. |

**From a file:**

```bash
sandlock skills put my-review --file ./prompts/review.md
```

**From stdin:**

```bash
cat ./prompts/setup.md | sandlock skills put setup-python
```

On success:

```
Skill "my-review" saved. It will be available as /my-review in every new sandbox.
```

Skills are upserted — putting a skill with an existing name replaces its content.

---

### `sandlock skills delete <name>`

Delete a skill.

```bash
sandlock skills delete <name>
```

```bash
sandlock skills delete my-review
```

The skill is removed from all future sessions. Sessions already running are not affected.

---

## How skills work

When you run `sandlock create`, the control plane fetches all of your stored skills and includes them in the claim request to the supervisor. The supervisor writes each skill to `/home/ubuntu/.claude/commands/<name>.md` before starting Claude Code.

This means every sandbox you open automatically has your skills available as slash commands — no manual setup needed inside the session.

## Writing a skill

Skills are standard Claude Code custom command files (Markdown). A minimal example:

```markdown
Review the staged changes in this repo for correctness, security issues, and test coverage. Summarize findings as a bulleted list.
```

A skill with arguments:

```markdown
Run the test suite for the $ARGUMENTS package and report any failures.
```

Use `$ARGUMENTS` as a placeholder for text typed after the slash command: `/run-tests auth` passes `auth` as `$ARGUMENTS`.

See the [Claude Code documentation](https://docs.anthropic.com/claude-code) for the full custom command syntax.

## Example workflow

```bash
# Write a skill file
cat > review.md << 'EOF'
Review the staged diff carefully. Check for: correctness bugs, missing error handling, security issues. Output a short bulleted list grouped by severity.
EOF

# Upload it
sandlock skills put code-review --file review.md

# Verify it's stored
sandlock skills list

# Start a session — /code-review will be available immediately
sandlock create --repo https://github.com/my-org/my-repo --use-stored-key
```
