# Memory

NeoClaw remembers things across conversations. This document explains how the memory system works and how to customize it.

---

## How memory works

NeoClaw has two kinds of memory:

**Long-term memory** (`memory.md`) — a curated list of facts the bot builds up over time. Things like your preferences, ongoing projects, and important context. The bot adds to this automatically when it learns something worth keeping.

**Daily logs** — a timestamped record of each day's conversations. Today's log and recent entries are automatically included in the bot's context so it remembers what you discussed recently.

Both live as plain markdown files in your data directory, so you can read and edit them directly.

---

## Directory layout

```
~/.neoclaw/
├── config.toml
└── data/
    └── agents/
        └── default/
            ├── SOUL.md          ← Agent personality and instructions (edit this)
            ├── USER.md          ← Optional user profile (edit this)
            ├── memory/
            │   ├── memory.md    ← Long-term memory facts
            │   └── daily/
            │       ├── 2026-02-23.md   ← Today's log
            │       ├── 2026-02-22.md
            │       └── ...
            ├── workspace/       ← Sandboxed file workspace
            ├── sessions/        ← Conversation history
            └── jobs.json        ← Scheduled jobs
```

---

## Long-term memory

`memory.md` is a structured markdown file that the bot uses to store facts it wants to remember permanently. It's organized into sections:

```markdown
# Memory

## User
- Name: Alex
- Timezone: America/Los_Angeles
- Works at Acme Corp

## Preferences
- Prefers concise responses
- Uses VS Code
- Vegetarian

## People
- Sarah (manager) — weekly 1:1 on Thursdays

## Ongoing
- Migrating the API to Go — deadline end of Q1
- Apartment hunting in Oakland (budget $2,500/month)
```

The bot adds to this automatically when you tell it something worth remembering:

> *"Remember that I prefer bullet points over long paragraphs"*

It also picks up on things naturally — if you mention your timezone in passing, it'll note it without being asked.

You can read the current memory with `/memory`, or open the file directly:

```bash
cat ~/.neoclaw/data/agents/default/memory/memory.md
```

You can edit it freely — add sections, clean up outdated entries, or remove anything you don't want the bot to know.

---

## Daily logs

Each day, the bot appends entries to a dated log file as the conversation progresses:

```
~/.neoclaw/data/agents/default/memory/daily/2026-02-23.md
```

Example contents:

```markdown
# Sunday, February 23, 2026

- 09:14: Discussed the API migration timeline. Decision: extend deadline to March 15.
- 11:30: Asked to set up a daily standup reminder at 9am on weekdays.
- 15:02: Reviewed draft email to Sarah about the budget request.
```

**What gets injected into context:** The last 24 hours of daily logs are automatically included with every request. Older logs are available via the `search_logs` tool if you ask the bot to look something up.

You can adjust how far back logs are injected in `config.toml`:

```toml
[context]
daily_log_lookback = "24h"   # or "48h", "12h", etc.
```

---

## SOUL.md — the agent's personality

`SOUL.md` is the file that defines the bot's personality, working style, and any standing instructions you want it to follow. It's injected into the system prompt on every request.

Its location:

```
~/.neoclaw/data/agents/default/SOUL.md
```

The default content created on first run:

```markdown
# Soul

## Persona
You are a helpful personal assistant.

## Preferences


## Tool Conventions
- Use tools when they improve accuracy.
- Keep outputs concise and actionable.
```

### Customizing SOUL.md

This is the most powerful way to shape how the bot behaves. You can:

- Change the persona ("You are a senior software engineer who prefers terse, technical responses.")
- Set communication preferences ("Always use bullet points. Never use corporate jargon.")
- Add standing context ("I am a freelance developer working primarily in Go and Python.")
- Define rules ("Never suggest using a new dependency without asking first.")
- Add expertise context ("I have 10 years of backend experience. Skip basic explanations.")

Example customized SOUL.md:

```markdown
# Soul

## Persona
You are a senior engineer and direct collaborator. Responses should be concise and
technical. Skip basic explanations. Prefer code examples over prose.

## User
- Backend developer, primarily Go and Python
- Works across multiple client projects
- Based in San Francisco (PST)

## Preferences
- Bullet points over paragraphs
- Short responses unless detail is explicitly requested
- Always ask before adding new dependencies

## Tool Conventions
- Run commands with confirmation when changes are irreversible
- Prefer reading files before modifying them
```

There are no rules about what goes in SOUL.md — it's a free-form instruction file. Write it the way you'd brief a new colleague.

---

## USER.md — your profile

`USER.md` is where you describe yourself. It's injected into the system prompt alongside SOUL.md on every request, giving the bot context about who you are without mixing it into the agent's personality and rules.

```
~/.neoclaw/data/agents/default/USER.md
```

The default content created on first run:

```markdown
# User

## Profile


## Preferences


## Current Context

```

Fill it in with whatever you want the bot to always know about you:

```markdown
# User

## Profile
- Software engineer, 10 years experience
- Primarily Go and Python
- Freelancer working across multiple client projects
- Based in San Francisco (PST)

## Preferences
- Concise responses, bullet points over prose
- Skip basic explanations
- Always ask before adding new dependencies

## Current Context
- Currently focused on migrating a monolith to microservices
- Client deadline: end of Q1
```

The split between SOUL.md and USER.md is intentional: SOUL.md controls how the bot behaves, USER.md tells it about you. Keeping them separate makes both easier to maintain.

---

## Memory tools

The bot has built-in tools for managing memory. You can use these directly in conversation:

| Tool | What it does |
|---|---|
| `memory_read` | Read the full contents of `memory.md` |
| `memory_append` | Add a fact to a section in `memory.md` |
| `memory_remove` | Remove a matching fact from `memory.md` |
| `daily_log` | Append an entry to today's daily log |
| `search_logs` | Search past daily logs for something specific |

You generally don't need to invoke these directly — the bot uses them automatically. But you can ask:

> *"What do you know about me?"* → bot calls `memory_read`

> *"Forget that I'm vegetarian"* → bot calls `memory_remove`

> *"What were we working on last week?"* → bot calls `search_logs`

---

## Resetting memory

To clear the conversation history without affecting memory:

```
/new
```

To clear long-term memory, edit or delete `memory.md` directly:

```bash
echo "# Memory\n" > ~/.neoclaw/data/agents/default/memory/memory.md
```

Daily logs are never automatically deleted. Archive or remove old ones manually from `~/.neoclaw/data/agents/default/memory/daily/` if needed.
