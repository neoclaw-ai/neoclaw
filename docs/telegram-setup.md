# Telegram Setup

NeoClaw runs as a Telegram bot. This guide covers creating a bot, getting a token, and pairing your Telegram account.

---

## Step 1 — Create a bot with BotFather

Telegram bots are created through [@BotFather](https://t.me/BotFather), an official Telegram bot that manages bot accounts.

1. Open Telegram and search for `@BotFather`, or go to [t.me/BotFather](https://t.me/BotFather).
2. Send the command `/newbot`.
3. BotFather will ask for a **display name** — this is what people see in chat (e.g. `My Assistant`).
4. Then it asks for a **username** — this must end in `bot` (e.g. `myassistant_bot`). Usernames are unique across all of Telegram.
5. BotFather will respond with your bot token. It looks like this:

   ```
   123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw
   ```

   **Keep this token private.** Anyone with it can control your bot.

---

## Step 2 — Add the token to your config

Open `~/.neoclaw/config.toml` and paste your token:

```toml
[channels.telegram]
enabled = true
token   = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
```

---

## Step 3 — Pair your Telegram account

NeoClaw uses a pairing flow to authorize which Telegram users can talk to the bot. Until you've paired, the bot ignores all messages.

> **Important:** The server must not be running when you pair. If `claw start` is active, stop it first.

Run the pairing command:

```bash
claw pair
```

You'll see something like:

```
Waiting for a Telegram message (timeout: 15 minutes)...
Send any message to your bot on Telegram to begin.
```

**In Telegram:** Find your bot by its username and send it any message — "hello" works fine.

NeoClaw will respond to you in Telegram with a 6-digit code:

```
Your pairing code: 847291
```

**Back in your terminal:** Type that code and press Enter.

```
Enter the pairing code from Telegram: 847291
✓ Paired successfully. User @yourname has been authorized.
```

Your Telegram account is now authorized. You can run `claw pair` again at any time to add additional users.

---

## Step 4 — Start the bot

```bash
claw start
```

Send your bot a message on Telegram to confirm everything is working.

---

## Adding more users

To authorize another Telegram user (a family member, colleague, etc.):

1. Stop the server: `Ctrl+C` or `systemctl --user stop neoclaw`
2. Run `claw pair` again
3. Have the new user send a message to the bot
4. They'll receive a code; you type it in the terminal
5. Start the server again

Each authorized user gets their own separate conversation history. Memory and scheduled jobs are shared.

---

## Troubleshooting

**The bot doesn't respond to my messages.**

- Confirm the server is running (`claw start` or check `systemctl --user status neoclaw`).
- Make sure you've paired your account. Run `claw pair` to add yourself if you haven't.
- Check that the token in your config is correct — copy it fresh from BotFather if unsure.

**`claw pair` times out without receiving a message.**

- Make sure you're messaging the right bot (search by the exact username you gave it).
- The bot must have its token configured before pairing. Run `claw start` once to validate config, then stop it before running `claw pair`.

**I get "token is required" when starting.**

- Make sure `channels.telegram.token` is set in `~/.neoclaw/config.toml`.
- Check that `channels.telegram.enabled` is set to `true`.
