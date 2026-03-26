# Channel Setup Guide

Connect your clawctl instances to messaging platforms. Each OpenClaw instance can run **multiple channels simultaneously** — one agent reachable on WhatsApp, Telegram, Discord, Slack, and more, with unified conversation history.

## How It Works

```
You (laptop)
  └─ SSH tunnel → clawctl server
                    └─ clawctl manages instances (Docker containers)
                         └─ Each container runs an OpenClaw gateway
                              └─ Gateway connects to channels
                                   ├─ WhatsApp (Baileys)
                                   ├─ Telegram (grammY)
                                   ├─ Discord (discord.js)
                                   ├─ Slack (Bolt)
                                   ├─ Signal (signal-cli)
                                   └─ ...30+ more
```

clawctl doesn't talk to channels directly. It manages the containers; OpenClaw inside the container handles channel connections.

## Two Ways to Configure Channels

### Option A: Interactive Wizard

```bash
clawctl channel <instance> <channel-type>
```

This runs OpenClaw's interactive setup wizard inside the container. It prompts for tokens, policies, and options.

### Option B: Direct Config

```bash
clawctl exec <instance> config set channels.<channel>.<key> <value> --json
clawctl restart <instance>
```

Set config values directly and restart. Better for scripting and automation.

---

## Telegram

The simplest channel to set up. Takes about 2 minutes.

### Prerequisites
- A Telegram bot token from [@BotFather](https://t.me/BotFather)

### Setup

```bash
# Create bot: message @BotFather on Telegram, send /newbot, follow prompts
# Copy the token (looks like: 123456789:ABCdefGhIJKlmNOPQRSTuvwxyz)

# Configure
clawctl exec alice config set channels.telegram.enabled true --json
clawctl exec alice config set channels.telegram.botToken '"YOUR_TOKEN"' --json
clawctl exec alice config set channels.telegram.dmPolicy '"pairing"' --json
clawctl restart alice

# Message your bot on Telegram — it replies with a pairing code
# Approve it:
clawctl exec alice pairing approve telegram <CODE>
```

### Config Reference

```json5
{
  "channels": {
    "telegram": {
      "enabled": true,
      "botToken": "123456789:ABC...",
      "dmPolicy": "pairing",          // "pairing" | "allowlist" | "open"
      "allowFrom": ["+15551234567"],   // optional sender allowlist
      "groups": {
        "*": { "requireMention": true }  // require @mention in groups
      }
    }
  }
}
```

### Multi-Account Telegram

```json5
{
  "channels": {
    "telegram": {
      "defaultAccount": "personal",
      "accounts": {
        "personal": { "botToken": "111:aaa" },
        "work":     { "botToken": "222:bbb" }
      }
    }
  }
}
```

---

## WhatsApp

Uses the Baileys library (Web API). Requires QR code scanning from a phone.

### Prerequisites
- A phone number (separate from your personal number recommended)
- WhatsApp installed on that phone

### Setup

```bash
# Configure access policy first
clawctl exec alice config set channels.whatsapp.enabled true --json
clawctl exec alice config set channels.whatsapp.dmPolicy '"pairing"' --json
clawctl restart alice

# Link via QR (interactive — needs terminal)
clawctl exec alice channels login --channel whatsapp
# A QR code appears in the terminal — scan it with WhatsApp:
#   WhatsApp → Settings → Linked Devices → Link a Device

# Message the WhatsApp number, approve pairing
clawctl exec alice pairing approve whatsapp <CODE>
```

### Config Reference

```json5
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "dmPolicy": "pairing",
      "allowFrom": ["+15551234567"],        // phone numbers that can DM
      "groupPolicy": "allowlist",           // "open" | "allowlist" | "disabled"
      "groupAllowFrom": ["+15551234567"]    // who can add bot to groups
    }
  }
}
```

### Tips
- Use a separate phone number — the bot takes over that WhatsApp account
- The QR link expires; re-run `channels login` if it times out
- WhatsApp sessions persist across restarts (stored in `credentials/`)

---

## Discord

Uses the Discord Bot API with Gateway (WebSocket).

### Prerequisites
1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Go to **Bot** → click **Reset Token** → copy the token
4. Enable **Privileged Gateway Intents**:
   - Message Content Intent
   - Server Members Intent
5. Go to **OAuth2** → **URL Generator**:
   - Scopes: `bot`, `applications.commands`
   - Bot Permissions: View Channels, Send Messages, Read Message History, Embed Links, Attach Files, Add Reactions
6. Copy the generated URL, open it, and invite the bot to your server

### Setup

```bash
clawctl exec alice config set channels.discord.enabled true --json
clawctl exec alice config set channels.discord.token '"YOUR_BOT_TOKEN"' --json
clawctl exec alice config set channels.discord.dmPolicy '"pairing"' --json
clawctl restart alice

# DM the bot on Discord, approve pairing
clawctl exec alice pairing approve discord <CODE>
```

### Config Reference

```json5
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "dmPolicy": "pairing",
      "groups": {
        "*": { "requireMention": true }
      }
    }
  }
}
```

### Tips
- Enable Developer Mode in Discord (Settings → Advanced) to copy server/channel/user IDs
- The bot needs **Send Messages** permission in any channel it should respond in
- In server Privacy Settings, ensure DMs from server members are enabled

---

## Slack

Uses Slack's Bolt SDK. Supports Socket Mode (recommended) or HTTP Events API.

### Prerequisites (Socket Mode)
1. Go to [Slack API](https://api.slack.com/apps) → Create New App
2. **Socket Mode**: enable it, create an App-Level Token (`xapp-...`) with `connections:write` scope
3. **Event Subscriptions**: enable and subscribe to bot events:
   - `app_mention`
   - `message.im`
   - `message.channels`
   - `message.groups`
   - `message.mpim`
   - `reaction_added`
4. **OAuth & Permissions**: add Bot Token Scopes:
   - `chat:write`, `channels:history`, `groups:history`, `im:history`, `mpim:history`
   - `channels:read`, `groups:read`, `im:read`, `users:read`
   - `reactions:read`, `files:read`, `files:write`
5. **Install App** to workspace → copy Bot User OAuth Token (`xoxb-...`)
6. **App Home**: enable Messages Tab

### Setup

```bash
clawctl exec alice config set channels.slack.enabled true --json
clawctl exec alice config set channels.slack.mode '"socket"' --json
clawctl exec alice config set channels.slack.appToken '"xapp-..."' --json
clawctl exec alice config set channels.slack.botToken '"xoxb-..."' --json
clawctl restart alice
```

Slack doesn't use pairing codes — access is controlled by workspace membership and channel invitation.

### Config Reference

```json5
{
  "channels": {
    "slack": {
      "enabled": true,
      "mode": "socket",         // "socket" (recommended) or "http"
      "appToken": "xapp-...",   // App-Level Token (Socket Mode)
      "botToken": "xoxb-...",   // Bot User OAuth Token
      "groups": {
        "*": { "requireMention": true }
      }
    }
  }
}
```

---

## Signal

Uses signal-cli (Java-based CLI client).

### Prerequisites
- Java runtime (for signal-cli)
- signal-cli installed: https://github.com/AsamK/signal-cli
- A phone number (can share with personal Signal via "linked device")

### Setup

```bash
# Option A: Link to existing Signal account (recommended)
signal-cli link -n "OpenClaw" --uri
# Scan the QR code with Signal app: Settings → Linked Devices

# Option B: Register a new number
signal-cli -u +15551234567 register
signal-cli -u +15551234567 verify <SMS_CODE>

# Configure
clawctl exec alice config set channels.signal.enabled true --json
clawctl exec alice config set channels.signal.account '"+15551234567"' --json
clawctl exec alice config set channels.signal.cliPath '"signal-cli"' --json
clawctl exec alice config set channels.signal.dmPolicy '"pairing"' --json
clawctl restart alice

# Message the Signal number, approve pairing
clawctl exec alice pairing approve signal <CODE>
```

### Config Reference

```json5
{
  "channels": {
    "signal": {
      "enabled": true,
      "account": "+15551234567",
      "cliPath": "signal-cli",      // or full path
      "dmPolicy": "pairing",
      "allowFrom": ["+15557654321"]
    }
  }
}
```

---

## Multi-Channel on One Instance

An instance can connect to all channels simultaneously:

```json5
// ~/.openclaw/alice/openclaw.json
{
  "channels": {
    "telegram": { "enabled": true, "botToken": "..." },
    "whatsapp": { "enabled": true, "dmPolicy": "pairing" },
    "discord":  { "enabled": true, "token": "..." },
    "slack":    { "enabled": true, "botToken": "xoxb-...", "appToken": "xapp-..." },
    "signal":   { "enabled": true, "account": "+15551234567" }
  }
}
```

One agent identity, reachable everywhere, with shared conversation memory.

---

## Team Setup

Give each team member their own agent with different channels:

```bash
clawctl group create team

# Alice: WhatsApp + Telegram
clawctl create team/alice
clawctl channel team/alice whatsapp
clawctl channel team/alice telegram

# Bob: Slack + Discord
clawctl create team/bob
clawctl channel team/bob slack
clawctl channel team/bob discord

# Start all
clawctl start-all

# Share skills and workspace across the group
clawctl group shared team --all
```

---

## Security: DM Pairing

By default, every channel uses **DM pairing** — unknown senders can't get responses until approved.

```bash
# Check pending pairing requests
clawctl exec alice pairing list

# Approve a request
clawctl exec alice pairing approve <channel> <CODE>

# Check who's approved
clawctl exec alice channels status --probe
```

### DM Policies

| Policy | Behavior |
|--------|----------|
| `pairing` | Unknown senders get a one-time approval code (default, recommended) |
| `allowlist` | Only pre-approved phone numbers/user IDs can DM |
| `open` | Anyone can DM (not recommended for public bots) |

### Group Policies

| Policy | Behavior |
|--------|----------|
| `open` | Respond to all group messages |
| `allowlist` | Only respond in allowlisted groups |
| `disabled` | Ignore all group messages |

By default, group messages require `@mention` (`requireMention: true`).

---

## Troubleshooting

```bash
# Check channel connectivity
clawctl exec alice channels status --probe

# View channel-specific logs
clawctl exec alice channels logs --channel telegram

# Full gateway logs
clawctl logs alice -f

# Re-login (e.g., WhatsApp session expired)
clawctl exec alice channels login --channel whatsapp

# Health check
clawctl health alice
```

### Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Bot doesn't respond to DMs | DM pairing required | `clawctl exec <name> pairing approve <channel> <CODE>` |
| Bot doesn't respond in groups | Mention required | `@mention` the bot, or set `requireMention: false` |
| WhatsApp QR expired | Session timeout | Re-run `channels login --channel whatsapp` |
| "Token invalid" on Telegram | Bot token changed/revoked | Get new token from @BotFather, update config |
| Discord bot offline | Missing intents | Enable Message Content + Server Members intents in developer portal |
| Slack not connecting | Socket Mode disabled | Enable Socket Mode in Slack app settings |

---

## Additional Channels

OpenClaw supports 30+ channels including:

- iMessage/BlueBubbles (macOS only)
- Google Chat
- IRC
- Microsoft Teams
- Matrix
- Mattermost
- Nextcloud Talk
- Twitch
- Line
- Zalo

These are available as extensions. Install via `clawctl exec <name> plugins install <channel>` and configure similarly to the core channels above.

For full per-channel documentation, see the OpenClaw docs at `docs/channels/` in the OpenClaw repository.
