# Security

NeoClaw is built around the idea that giving an AI assistant access to your server is a meaningful security decision. This document explains how NeoClaw limits what the bot can do and keeps you in control.

---

## Overview: five layers of defense

NeoClaw uses multiple independent layers of protection. Each one stops a different category of problem.

| Layer | What it does |
|---|---|
| **Telegram allowlist** | Only authorized Telegram accounts can talk to the bot |
| **Command approval** | Shell commands need your approval before they run |
| **Trust hierarchy** | Owner instructions always override anything else |
| **Process sandbox** | The bot process is isolated at the OS level |
| **Network proxy** | Outbound network access from commands is filtered |

These layers work independently. If one is bypassed, the others still apply.

---

## Security modes

Set the overall security mode in `config.toml`:

```toml
[security]
mode = "standard"   # or "strict" or "danger"
```

| Mode | Approval prompts | Read access | Write access | Network proxy |
|---|---|---|---|---|
| `standard` | Yes | Full filesystem | `~/.neoclaw/` only | Applied |
| `strict` | Yes | Workspace + system dirs only | Workspace only | Applied |
| `danger` | None — auto-approved | Unrestricted | Unrestricted | Not applied |

**`standard`** is the recommended mode for most users. The bot can read files anywhere on your system to help you work, but can only write inside its own data directory.

**`strict`** restricts read access as well. The bot can only read files inside its own workspace directory and standard system binary paths. This is the safest option, but it means the bot can't read your code, configs, or other files outside the workspace.

> **Note:** `strict` mode requires a compatible kernel on Linux. NeoClaw will refuse to start if it can't apply the sandbox. On macOS, `strict` mode is always available.

**`danger`** disables all approval prompts and sandbox protections. Commands run without asking, and the network proxy is not started. Only use this in a fully trusted local environment.

---

## Layer 1 — Telegram allowlist

Every message sent to the bot is checked against a list of authorized Telegram user IDs. Messages from anyone not on the list are silently ignored — the sender gets no response and no indication the bot exists.

You add users to this list using `claw pair`. The list is stored in:

```
~/.neoclaw/data/policy/allowed_users.json
```

---

## Layer 2 — Command approval

Before running any shell command (`run_command` tool), NeoClaw checks the command against a policy file. There are three possible outcomes:

1. **Auto-approved** — the command matches a pattern on your allow list and runs immediately.
2. **Prompted** — the command is unknown; you get a Telegram message with [✅ Approve] and [❌ Deny] buttons.
3. **Blocked** — the command matches a pattern on your deny list and is refused.

When you approve or deny a command, the decision is saved permanently. NeoClaw generates a pattern from the command (e.g. `git commit *`) so that similar future commands are handled the same way without prompting again.

### The policy file

Command rules live in:

```
~/.neoclaw/data/policy/allowed_commands.json
```

Example:

```json
{
  "allow": [
    "ls *",
    "cat *",
    "git status *",
    "git commit *",
    "npm run *"
  ],
  "deny": [
    "git push --force *",
    "rm -rf *"
  ]
}
```

Rules are evaluated **deny first, then allow**. If a command matches a deny rule, it's blocked regardless of the allow list. If it matches neither, you're prompted.

The default allow list (created on first run) includes common read-only commands: `ls`, `cat`, `grep`, `find`, `curl`, and others. You'll be prompted the first time any other command is attempted.

### Pattern syntax

- Patterns are matched token by token against the command.
- `*` matches any number of tokens and can appear anywhere in the pattern.
- `git commit *` matches `git commit -m "my message"`.
- `git * main` matches `git checkout main` and `git merge main`.

---

## Layer 3 — Trust hierarchy

The system prompt is structured so that your messages always take priority over any instructions the bot might receive from other sources (like third-party skills in the future). Owner instructions win. This is enforced at the prompt level.

---

## Layer 4 — Process sandbox

When the bot runs a shell command, that command runs inside an OS-level process sandbox.

**On Linux:** The sandbox uses kernel-level isolation to restrict what the process can read and write.

**On macOS:** The sandbox uses the built-in `sandbox-exec` facility.

See the [Security modes](#security-modes) table above for read and write access by mode.

---

## Layer 5 — Network allowlist

Shell commands that make network requests (e.g. `curl`) go through a local HTTP proxy that filters domains.

The domain policy lives in:

```
~/.neoclaw/data/policy/allowed_domains.json
```

Example:

```json
{
  "allow": [
    "api.github.com",
    "pypi.org"
  ],
  "deny": []
}
```

When a command tries to reach an unknown domain, you're prompted in Telegram. Approve or deny, and the decision is saved. A domain entry matches itself and all its subdomains — `github.com` also covers `api.github.com`.

The default allow list includes `api.anthropic.com`, `api.openrouter.ai`, and `api.search.brave.com`. Everything else is blocked until you approve it.

> **Note:** This proxy applies to subprocess commands (`run_command`). The bot's own web tools (`web_search`, `http_request`) check the domain list directly without the proxy.

---

## What is and isn't protected

NeoClaw protects your filesystem and network from unintended access. It does **not**:

- Protect against malicious content in files the bot reads (prompt injection via file content is a real risk on any AI assistant).
- Restrict the bot from reading files in `standard` mode — it can read your code, configs, and documents to help you work.
- Provide protection if you run in `danger` mode.

For the highest confidence, run in `strict` mode and review the allow lists before deploying on a sensitive server.
