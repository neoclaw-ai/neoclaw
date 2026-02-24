# Slash Commands

Slash commands are handled directly by NeoClaw — no LLM call, no token cost, instant response.

Send them as a message to the bot on Telegram, or type them in `claw cli`.

---

## Quick reference

| Command | Aliases | Description |
|---|---|---|
| `/new` | `/reset` | Clear the current session and start fresh |
| `/jobs` | | List scheduled jobs |
| `/usage` | | Show API spending summary |
| `/help` | | List all available commands |

---

## `/new` · `/reset`

Clears the current conversation history and starts fresh. Long-term memory and scheduled jobs are not affected.

```
/new
→ Session cleared. Starting fresh!
```

Use this when the conversation has gone off track, the context window is cluttered, or you simply want a clean slate.

---

## `/usage`

Shows how much you've spent on API calls today and this month.

```
/usage
→ Today:    12,400 tokens  ($0.18)
  Month:    340,000 tokens ($4.92)
```

If you've configured spending limits, they'll also appear:

```
/usage
→ Today:    12,400 tokens  ($0.18 / $5.00 limit)
  Month:    340,000 tokens ($4.92 / $50.00 limit)
```

---

## `/jobs`

Lists all scheduled jobs.

```
/jobs
→ Scheduled jobs (2):

  1. daily-standup  (0 9 * * 1-5)  ✓ enabled
     "Good morning! What's on the agenda today?"

  2. weekly-review  (0 17 * * 5)   ✓ enabled
     "Send me a summary of what I accomplished this week"
```

To create, update, or delete jobs, ask the bot in natural language:

> *"Remind me every weekday at 9am to check my calendar"*

> *"Delete the weekly-review job"*

---

## `/help`

Lists all available slash commands.

```
/help
→ Available commands:
  /new, /reset  — Clear session, start fresh
  /jobs         — List scheduled jobs
  /usage        — Show spending summary
  /help         — Show this message
```
