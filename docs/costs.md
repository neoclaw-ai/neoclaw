# Managing costs

NeoClaw is designed to be dramatically cheaper than OpenClaw, but your actual spend depends on how much you use it and how you configure it. This page covers the tools available to keep costs under control.

---

## Spending limits

The most direct way to cap costs is to set hard limits in your config. NeoClaw checks these before every LLM call and stops if you've exceeded them.

```toml
[costs]
daily_limit   = 5.0    # USD — 0 = disabled
monthly_limit = 50.0   # USD — 0 = disabled
```

When a limit is hit, the bot tells you and stops making API calls until the period resets. You won't be surprised by a large bill at the end of the month.

Check your current spend at any time with `/usage`:

```
/usage
→ Today:    12,400 tokens  ($0.18 / $5.00 limit)
  Month:    340,000 tokens ($4.92 / $50.00 limit)
```

---

## Context window size

The single biggest lever on per-request cost is how many tokens are sent with each message. NeoClaw automatically summarizes old conversation history, but you control the budget.

```toml
[context]
max_tokens = 10000   # default
```

Lowering this means older messages get summarized sooner, reducing what's sent with each request. The trade-off is that the bot has less immediate conversation context to work with.

| Setting | Effect |
|---|---|
| `10000` (default) | Good balance for most use |
| `4000` | Aggressive summarization, lower cost |
| `16000` | More context, higher cost |

The `recent_messages` setting controls how many of the most recent messages are always kept verbatim regardless of the token budget:

```toml
[context]
recent_messages = 12   # default
```

Reducing this (e.g. to `5`) means fewer messages are preserved before summarization kicks in.

---

## Tool output truncation

When the agent runs a tool — reading a file, running a command, fetching a URL — the output is stored in conversation history and sent with subsequent requests. Large outputs inflate costs fast.

```toml
[context]
tool_output_length = 2500   # characters — default
```

When a tool output exceeds this limit, the full output is saved to a temp file and only a truncated version is kept in history. Lowering this reduces how much tool output accumulates in the context window.

For most tasks `2500` characters is sufficient. If the agent frequently needs to reference large outputs, you can raise it — but be aware of the cost impact.

---

## Daily log injection

NeoClaw automatically injects recent daily log entries into every request so the bot remembers what you've been working on. The default is 12 hours.

```toml
[context]
daily_log_lookback = "12h"   # default
```

If your daily logs are long (many conversations throughout the day), reducing this to `"6h"` or `"3h"` trims the injected context. Increase it to `"24h"` or `"48h"` if you want the bot to be aware of more recent history across sessions.

---

## Choosing a cheaper model

The fastest way to cut costs is to use a cheaper model. OpenRouter gives access to models that cost a fraction of Anthropic's pricing:

```toml
[llm.default]
provider = "openrouter"
api_key  = "sk-or-v1-..."
model    = "deepseek/deepseek-chat"   # ~$0.14/M tokens
```

For reference, `deepseek/deepseek-chat` costs roughly 50–100x less per token than Claude Sonnet. For everyday tasks like web search, file operations, and reminders, the quality difference is minimal.

See [Configuration](configuration.md) for the full list of provider and model options.

---

## Putting it together

A cost-optimized config for light daily use:

```toml
[costs]
daily_limit   = 2.0
monthly_limit = 20.0

[context]
max_tokens         = 2000
recent_messages    = 5
tool_output_length = 1000
daily_log_lookback = "12h"
```

With these settings and a cheap OpenRouter model, typical usage costs well under $5/month.
