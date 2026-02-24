# 🐊 NeoClaw

**NeoClaw is a secure, stable, and powerful AI assistant**

[![Release](https://img.shields.io/github/v/release/neoclaw-ai/neoclaw?style=flat-square)](https://github.com/neoclaw-ai/neoclaw/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Go Report Card](https://goreportcard.com/badge/github.com/neoclaw-ai/neoclaw?style=flat-square)](https://goreportcard.com/report/github.com/neoclaw-ai/neoclaw)
[![Build](https://img.shields.io/github/actions/workflow/status/neoclaw-ai/neoclaw/ci.yml?branch=main&style=flat-square)](https://github.com/neoclaw-ai/neoclaw/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)

NeoClaw is a self-hosted AI assistant that runs as a Telegram bot on your own server. No subscription, no SaaS — just a single binary, your API key, and full control. It connects to Anthropic directly, or to any model on [OpenRouter](https://openrouter.ai).

NeoClaw is a lightweight, secure alternative to OpenClaw. OpenClaw sends your full conversation history with every request — sessions that grow to 50,000+ tokens — which is why users regularly see $50–200/month API bills. NeoClaw's context management and built-in cost controls bring that to $3–15/month.

NeoClaw is hardened with battle-tested security by default. Your bot cannot delete your important files. You're in complete control over what commands run and what websites are accessed. 

**[→ Get started in 5 minutes](#-quick-start)**

---

## ⚡ Simple to run

- **One binary, no dependencies.** Copy it to any Linux or macOS machine and it runs. No Docker, no package manager, no root access required.
- **Runs anywhere.** A $5/month VPS, a spare Raspberry Pi, your old laptop — if it runs Linux or macOS, it works.
- **Sandboxing included.** Process isolation and network filtering are built in at the OS level. No containers needed.

→ [Full Quick Start](docs/quick-start.md) · [Telegram Setup](docs/telegram-setup.md)

---

## 🔐 Secure by default

An AI assistant with shell access is a real security surface. NeoClaw is built with that in mind.

- **Allowlisted users only.** Only Telegram accounts you've authorized can talk to the bot. Everyone else is silently ignored.
- **You approve commands.** Before running any shell command, NeoClaw asks via Telegram inline keyboard. Approved patterns are remembered — you won't be asked twice.
- **OS-level sandbox.** The bot process is isolated at the kernel level. It can only write inside its own workspace. Your SSH keys, home folder, and personal files are off limits.

→ [Security documentation](docs/security.md)

---

## 💰 Up to 90% cheaper than OpenClaw

- **Smart context window.** Automatic summarization keeps conversation history around ~3,300 tokens per request instead of 50,000+.
- **No heartbeat LLM calls.** Scheduled jobs use a deterministic cron scheduler — no tokens burned just to check "is it time yet?"
- **Built-in spend limits.** Set a daily or monthly cap. NeoClaw stops making API calls if you hit it.

Works with Anthropic directly, or through OpenRouter for access to DeepSeek, Mistral, Llama, and 100+ other models — including options under $1/million tokens.

→ [Managing costs](docs/costs.md)

---

## 🚀 Quick Start

→ [Full Quick Start](docs/quick-start.md) · [Telegram Setup](docs/telegram-setup.md)

**1. Download**

| Platform | Download |
|---|---|
| Linux x86-64 | [neoclaw-linux-amd64.tar.gz](https://github.com/neoclaw-ai/neoclaw/releases/latest/download/neoclaw-linux-amd64.tar.gz) |
| Linux ARM64 | [neoclaw-linux-arm64.tar.gz](https://github.com/neoclaw-ai/neoclaw/releases/latest/download/neoclaw-linux-arm64.tar.gz) |
| macOS (Apple Silicon) | [neoclaw-darwin-arm64.tar.gz](https://github.com/neoclaw-ai/neoclaw/releases/latest/download/neoclaw-darwin-arm64.tar.gz) |
| macOS (Intel) | [neoclaw-darwin-amd64.tar.gz](https://github.com/neoclaw-ai/neoclaw/releases/latest/download/neoclaw-darwin-amd64.tar.gz) |

Windows: Run the Linux binary under WSL2.

```bash
tar -xzf neoclaw-*.tar.gz
sudo mv claw /usr/local/bin/
```

**2. Configure**

Run `claw start` once — it creates `~/.neoclaw/config.toml` and exits with setup instructions. Open that file and fill in two values:

```toml
[llm.default]
api_key = "your-anthropic-api-key"

[channels.telegram]
token = "your-telegram-bot-token"   # from @BotFather
```

**3. Pair and start**

```bash
claw pair    # authorize your Telegram account (one-time)
claw start   # bot is live
```

Open Telegram and start chatting.

---

## 🤖 What can it do?

Send messages like these from Telegram:

```
Summarize the top Hacker News posts from today
```
```
Write a bash script to rename all the JPGs in ~/photos by date, then run it
```
```
Remind me every weekday at 9am to check my OKRs
```
```
What did we discuss last Tuesday?
```
```
Remember that my server IP is 10.0.0.1
```

NeoClaw has 15 built-in tools: file read/write, shell commands, web search, HTTP requests, memory, and scheduled jobs. It remembers facts across conversations and can run recurring tasks without any heartbeat polling.

Want to talk to it without Telegram? `claw cli` drops you into a local terminal session.

---

## 📚 Documentation

| | |
|---|---|
| [Quick Start](docs/quick-start.md) | Install, configure, and get running in 5 minutes |
| [Telegram Setup](docs/telegram-setup.md) | Create a bot with BotFather and pair your account |
| [Security](docs/security.md) | Security model, approval flows, and permission files |
| [Memory](docs/memory.md) | How NeoClaw remembers things across conversations |
| [Configuration](docs/configuration.md) | Complete configuration reference |
| [Commands](docs/commands.md) | Slash command quick reference |
| [Costs](docs/costs.md) | Spending limits and cost optimization |

---

## Building from source

Requires Go 1.21+.

```bash
go install github.com/neoclaw-ai/neoclaw/cmd/claw@latest
```

Or clone and build:

```bash
git clone https://github.com/neoclaw-ai/neoclaw.git
cd neoclaw
go build -o bin/claw ./cmd/claw
```

---

## Contributing

Issues and pull requests welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT — see [LICENSE](LICENSE).
