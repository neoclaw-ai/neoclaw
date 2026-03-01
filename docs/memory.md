# Memory

NeoClaw remembers things across conversations. This document explains how the memory system works and how to customize it.

---

## How memory works

NeoClaw has two kinds of memory:

**Persistent facts** (`memory.tsv`) — things the bot should know as current state in every future conversation: your identity, preferences, timezone, tool settings, and time-bounded facts like travel plans. The bot adds and updates these automatically. Old facts are superseded by new ones on the same topic — only the latest is used.

**Daily logs** (`daily/*.tsv`) — a record of what happened each day: meetings, tasks, decisions, follow-ups, observations. One file per calendar day. Recent days are automatically included in context; older days are searchable on demand.

Both are plain TSV (tab-separated) files. You can inspect them with standard tools:

```bash
cat ~/.neoclaw/data/agents/default/memory/memory.tsv
grep location ~/.neoclaw/data/agents/default/memory/memory.tsv
grep followup ~/.neoclaw/data/agents/default/memory/daily/2026-02-28.tsv
```

---

## Directory layout

```
~/.neoclaw/
├── config.toml
└── data/
    └── agents/
        └── default/
            ├── SOUL.md              <- Agent personality and instructions (edit this)
            ├── USER.md              <- Your profile (edit this)
            ├── memory/
            │   ├── memory.tsv       <- Persistent facts
            │   └── daily/
            │       ├── 2026-02-28.tsv   <- Today's log
            │       ├── 2026-02-27.tsv
            │       └── ...
            ├── workspace/           <- Sandboxed file workspace
            ├── sessions/            <- Conversation history
            └── jobs.json            <- Scheduled jobs
```

---

## Persistent facts

`memory.tsv` stores facts the bot wants to remember across conversations. Each line has four tab-separated columns:

```
ts	tags	text	kv
2026-02-25T22:23:50.000000000-08:00	location	Lives in New York	-
2026-02-26T10:00:00.000000000-08:00	diet	Lactose intolerant	-
2026-02-27T18:00:00.000000000-08:00	location	In SF until Friday	expires=1740787200
```

**How it works:**

- The bot writes facts automatically when you mention something worth keeping — your timezone, dietary preferences, work context, tool choices, behavioral preferences.
- Each fact has a **topic** (the first tag). When the bot learns something new about the same topic, it writes a new entry. Only the latest entry per topic is included in context — the old one stays in the file as history.
- Facts can have an **expiry**. A travel plan or hotel stay can be set to expire automatically. When it does, the system falls back to the previous non-expired fact for that topic. For example, "In SF until Friday" expires and the bot automatically sees "Lives in New York" again.
- The bot also stores behavioral instructions as facts. If you say "always respond in bullet points," it writes that as a persistent fact framed as an instruction to its future self.

**You can inspect and search the file directly:**

```bash
cat ~/.neoclaw/data/agents/default/memory/memory.tsv
grep diet ~/.neoclaw/data/agents/default/memory/memory.tsv
```

The file is append-only — the bot never modifies or deletes existing lines. To remove a fact, you can edit the file manually.

---

## Daily logs

Each day, the bot appends structured entries to a dated TSV file as the conversation progresses:

```
~/.neoclaw/data/agents/default/memory/daily/2026-02-28.tsv
```

Example contents:

```
ts	tags	text	kv
2026-02-28T09:14:00.000000000-08:00	decision,api	Extend API migration deadline to March 15	scope=project_api
2026-02-28T11:30:00.000000000-08:00	task,scheduling	Set up daily standup reminder at 9am weekdays	status=done
2026-02-28T15:02:00.000000000-08:00	event,email	Reviewed draft email to Sarah about budget request	actor=sarah
```

Each entry has a semantic type as its first tag (like `task`, `event`, `decision`, `followup`) and optional domain labels as additional tags.

**What gets injected into context:** Today's log and yesterday's log are automatically included in every request. Older logs are not injected but are searchable — ask the bot to look something up and it will search past logs automatically.

You can adjust how many days are injected in `config.toml`:

```toml
[context]
daily_log_lookback_days = 2   # today + yesterday (default)
```

Set to `1` for today only, or `3` to include two previous days.

When you run `/new` to start a new session, the bot writes a structured summary of the completed session to the daily log before clearing conversation history.

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

## Resetting memory

To clear the conversation history without affecting memory:

```
/new
```

This also writes a session summary to the daily log before clearing.

To clear persistent facts, delete or truncate `memory.tsv`:

```bash
echo -e "ts\ttags\ttext\tkv" > ~/.neoclaw/data/agents/default/memory/memory.tsv
```

Daily logs are never automatically deleted. Archive or remove old ones manually from `~/.neoclaw/data/agents/default/memory/daily/` if needed.
