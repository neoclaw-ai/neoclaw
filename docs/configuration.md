# Configuration Reference

NeoClaw is configured through a single TOML file at `~/.neoclaw/config.toml`. This file is created automatically on first run with sensible defaults.

To see your current merged configuration (including all defaults):

```bash
claw config
```

---

## `[llm.default]` — Language model

```toml
[llm.default]
provider        = "anthropic"
api_key         = "sk-ant-..."
model           = "claude-sonnet-4-6"
max_tokens      = 8192
request_timeout = "30s"
```

| Key | Default | Description |
|---|---|---|
| `provider` | `"anthropic"` | LLM provider. Options: `anthropic`, `openrouter` |
| `api_key` | *(required)* | API key. Supports `$ENV_VAR` expansion. |
| `model` | `"claude-sonnet-4-6"` | Model name. See provider docs for valid values. |
| `max_tokens` | `8192` | Maximum tokens the model may generate per response. |
| `request_timeout` | `"30s"` | How long to wait for an API response before giving up. |

### Provider: Anthropic

Use your API key from [console.anthropic.com](https://console.anthropic.com).

```toml
[llm.default]
provider = "anthropic"
api_key  = "sk-ant-..."
model    = "claude-sonnet-4-6"
```

Common Anthropic models:
- `claude-opus-4-6` — most capable, highest cost
- `claude-sonnet-4-6` — good balance (default)
- `claude-haiku-4-5-20251001` — fastest, lowest cost

### Provider: OpenRouter

[OpenRouter](https://openrouter.ai) gives access to 100+ models through a single API key, including cheaper alternatives.

```toml
[llm.default]
provider = "openrouter"
api_key  = "sk-or-v1-..."
model    = "deepseek/deepseek-chat"
```

Common OpenRouter models:
- `deepseek/deepseek-chat` — very low cost (~$0.14/M tokens)
- `mistralai/mistral-small` — fast and affordable
- `anthropic/claude-sonnet-4-6` — Anthropic via OpenRouter

---

## `[channels.telegram]` — Telegram bot

```toml
[channels.telegram]
enabled = true
token   = "123456789:AAH..."
```

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Set to `false` to disable the Telegram channel entirely. |
| `token` | *(required when enabled)* | Bot token from [@BotFather](https://t.me/BotFather). |

Authorized Telegram user IDs are managed separately via `claw pair` and stored in `~/.neoclaw/data/policy/allowed_users.json`. They are not part of `config.toml`.

---

## `[security]` — Sandbox and approvals

```toml
[security]
mode            = "standard"
command_timeout = "5m"
```

| Key | Default | Description |
|---|---|---|
| `mode` | `"standard"` | Security mode. Options: `standard`, `strict`, `danger`. See [Security docs](security.md). |
| `command_timeout` | `"5m"` | Maximum execution time for shell commands. Commands running longer are killed. |

**Mode reference:**

- `standard` — Approval prompts enabled. Bot can read the full filesystem, write only to `~/.neoclaw/`. Sandbox applied when available.
- `strict` — Approval prompts enabled. Read access restricted to workspace and system binaries. Startup fails if the sandbox is unavailable.
- `danger` — No approval prompts, no sandbox, no network proxy. Everything auto-approved. Use only in fully trusted environments.

---

## `[costs]` — Spending limits

```toml
[costs]
daily_limit   = 0.0
monthly_limit = 0.0
```

| Key | Default | Description |
|---|---|---|
| `daily_limit` | `0.0` | Soft daily spend limit in USD. `0` = disabled. Checked before each LLM call. |
| `monthly_limit` | `0.0` | Soft monthly spend limit in USD. `0` = disabled. |

When a limit is reached, NeoClaw stops making LLM calls and sends you a message explaining why. Use `/usage` to see current spending.

Limits are "soft" — a request that starts just under the limit can still complete and push you slightly over. NeoClaw checks the limit before each call, not during.

Example — set a $5/day limit:

```toml
[costs]
daily_limit = 5.0
```

---

## `[context]` — Context window management

```toml
[context]
max_tokens         = 10000
recent_messages    = 12
max_tool_calls     = 10
tool_output_length = 2500
daily_log_lookback = "12h"
```

| Key | Default | Description |
|---|---|---|
| `max_tokens` | `10000` | Token budget for conversation context. When history exceeds this, older messages are summarized. |
| `recent_messages` | `12` | Number of recent messages always kept verbatim, regardless of `max_tokens`. |
| `max_tool_calls` | `10` | Maximum tool-call iterations per message before the agent stops. |
| `tool_output_length` | `2500` | Maximum characters of tool output stored inline in history. Larger outputs are saved to a temp file. |
| `daily_log_lookback` | `"12h"` | How far back daily log entries are injected into the system prompt. |

**Tuning for cost:** Lowering `max_tokens` reduces the amount of history sent with each request, which lowers per-request token cost at the expense of the bot remembering less context.

**Tuning for memory:** Increasing `daily_log_lookback` (e.g. `"48h"`) injects more daily log history into context, so the bot is aware of more recent activity.

---

## `[web.search]` — Web search

```toml
[web.search]
provider = "brave"
api_key  = "BSA..."
```

| Key | Default | Description |
|---|---|---|
| `provider` | `""` | Search provider. Currently only `"brave"` is supported. Leave empty to disable. |
| `api_key` | `""` | API key for the search provider. |

Web search is **optional**. If `provider` is empty or `api_key` is not set, the `web_search` tool is disabled. All other tools continue to work normally.

To enable Brave Search:
1. Sign up at [brave.com/search/api](https://brave.com/search/api/).
2. The free tier includes 2,000 requests/month — sufficient for personal use.
3. Add your key to the config.

---

## Environment variables

### `NEOCLAW_HOME`

By default, NeoClaw stores all its data in `~/.neoclaw/`. Set `NEOCLAW_HOME` to use a different location:

```bash
export NEOCLAW_HOME=/opt/neoclaw
claw start
```

This is useful if you're running multiple instances, want data on a separate disk, or are deploying as a system service under a non-home directory.

### Config value expansion

String config values support `$ENV_VAR` expansion. This is useful for keeping secrets out of your config file:

```toml
[llm.default]
api_key = "$ANTHROPIC_API_KEY"
```

Set the variable in your shell or in a `.env` file loaded by your service manager.

---

## Full example config

```toml
[llm.default]
provider        = "anthropic"
api_key         = "$ANTHROPIC_API_KEY"
model           = "claude-sonnet-4-6"
max_tokens      = 8192
request_timeout = "30s"

[channels.telegram]
enabled = true
token   = "$TELEGRAM_BOT_TOKEN"

[security]
mode            = "standard"
command_timeout = "5m"

[costs]
daily_limit   = 5.0
monthly_limit = 50.0

[context]
max_tokens         = 10000
recent_messages    = 12
max_tool_calls     = 10
tool_output_length = 2500
daily_log_lookback = "12h"

[web.search]
provider = "brave"
api_key  = "$BRAVE_API_KEY"
```
