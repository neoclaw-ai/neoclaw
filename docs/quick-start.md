# Quick Start

Get NeoClaw running in about 5 minutes.

## Prerequisites

- A Linux VPS or Mac (macOS 12+). Windows users: use WSL2 with the Linux binary.
- An API key from [Anthropic](https://console.anthropic.com) or [OpenRouter](https://openrouter.ai/keys).
- A Telegram account. You'll create a bot in the next section.

---

## Step 1 — Download the binary

Download the binary for your platform from the [releases page](https://github.com/neoclaw-ai/neoclaw/releases/latest).

| Platform | File |
|---|---|
| Linux x86-64 | `claw-linux-amd64` |
| Linux ARM64 | `claw-linux-arm64` |
| macOS (Apple Silicon) | `claw-darwin-arm64` |
| macOS (Intel) | `claw-darwin-amd64` |

Make it executable and move it onto your PATH:

```bash
chmod +x claw-*
sudo mv claw-* /usr/local/bin/claw
```

Verify it works:

```bash
claw --version
```

### Building from source

If you prefer to build from source, you'll need Go 1.21+:

```bash
go install github.com/neoclaw-ai/neoclaw/cmd/claw@latest
```

---

## Step 2 — Create your config

Run any `claw` command once to initialize your data directory and create a starter config:

```bash
claw start
```

This will fail (your API key isn't set yet — that's expected). It creates `~/.neoclaw/config.toml` and exits with a message pointing you there.

Open that file in your editor:

```bash
nano ~/.neoclaw/config.toml
```

Fill in your API key and Telegram bot token. The two required fields are:

```toml
[llm.default]
api_key = "sk-ant-..."          # Your Anthropic API key

[channels.telegram]
token = "123456:ABC-..."        # Your Telegram bot token from @BotFather
```

Don't have a Telegram bot token yet? See [Telegram Setup](telegram-setup.md) — it takes about 2 minutes.

### Using a different LLM provider

To use OpenRouter instead of Anthropic directly:

```toml
[llm.default]
provider    = "openrouter"
api_key     = "sk-or-v1-..."
model       = "deepseek/deepseek-chat"   # or any OpenRouter model
```

OpenRouter gives you access to DeepSeek, Mistral, Llama, and 100+ other models through a single API key. Some models cost a fraction of Anthropic's pricing.

---

## Step 3 — Authorize your Telegram account

NeoClaw uses a pairing process to authorize which Telegram accounts can talk to the bot. This is a one-time setup.

Make sure the server is not running, then run:

```bash
claw pair
```

The command will tell you to send any message to your bot on Telegram. Do that. NeoClaw will send a 6-digit code back to you in Telegram. Enter that code in your terminal to complete the pairing.

See [Telegram Setup](telegram-setup.md) for the full walkthrough.

---

## Step 4 — Start the bot

```bash
claw start
```

Your bot is now live. Open Telegram and send it a message.

To run it in the background as a service, see the [Linux systemd](#running-as-a-service) section below.

---

## Trying it without Telegram

You can talk to NeoClaw directly from your terminal using `claw cli`. This is useful for testing your setup without going through Telegram:

```bash
# Interactive session
claw cli

# Single message (useful for scripting)
claw cli -p "what is the current date and time"
```

The CLI session has full access to all tools. It uses a separate conversation history from Telegram.

---

## Running as a service

On Linux, you can run NeoClaw as a systemd user service so it starts automatically on boot.

Create the service file:

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neoclaw.service << 'EOF'
[Unit]
Description=NeoClaw AI Assistant
After=network.target

[Service]
ExecStart=/usr/local/bin/claw start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF
```

Enable and start it:

```bash
systemctl --user enable neoclaw
systemctl --user start neoclaw
systemctl --user status neoclaw
```

---

## Optional — Enable web search

NeoClaw supports web search via the [Brave Search API](https://brave.com/search/api/). The free tier covers 2,000 searches per month, which is more than enough for personal use.

1. Sign up at [brave.com/search/api](https://brave.com/search/api/) and get a free API key.
2. Add it to your config:

```toml
[web.search]
provider = "brave"
api_key  = "BSA..."
```

Without this, the `web_search` tool is disabled. All other tools work normally.

---

## Next steps

- [Telegram Setup](telegram-setup.md) — Full walkthrough for creating a bot and pairing your account
- [Security](security.md) — Understanding how approvals and sandboxing work
- [Memory](memory.md) — How NeoClaw remembers things and how to customize its personality
- [Configuration](configuration.md) — All available configuration options
