# Discord Server Template — Kit + CampPowerUp Community

A reusable organization template for the Discord server shared by the **Kit AI bot** and the **CampPowerUp** project. Use it as-is for the community server, or as a starting point for future servers.

The machine-readable version of this template lives at [`config/discord-server-template.json`](../config/discord-server-template.json) and can be applied automatically — see [Applying the template](#applying-the-template).

---

## Roles

| Role | Color | Hoisted | Purpose |
|------|-------|---------|---------|
| **Admin** | 🔴 Red | Yes | Server owners. Full administrator permissions. |
| **Moderator** | 🔵 Blue | Yes | Community moderation: kick/ban, manage messages, timeout members, manage nicknames. |
| **Developer** | 🟣 Purple | Yes | Contributors to Kit or CampPowerUp. Member permissions + access to staff-only dev channels (e.g. `#bot-logs`). |
| **Member** | 🟢 Green | No | Default trusted role for community members. Read/write in open channels, connect/speak in voice. |
| **Bot** | ⚪ Gray | No | For Kit and other bots. Read/write, embed links, attach files, add reactions. |

**Role hierarchy** (top to bottom): Admin → Moderator → Developer → Bot → Member → @everyone.

> ⚠️ Keep the bot's own managed role **above** any roles it needs to assign, and note that a bot can only create/manage roles below its own highest role.

## Categories & Channels

### 📌 WELCOME & INFO — *read-only*

Everyone can read; only Admin/Moderator (and bots) can post.

| Channel | Type | Purpose |
|---------|------|---------|
| `#welcome` | Text | Intro to the server, what Kit and CampPowerUp are, how to get involved. |
| `#rules` | Text | Server rules and code of conduct. |
| `#announcements` | Text | Server-wide announcements from staff. |
| `#roles-info` | Text | What each role means and how to get one. |

### 💬 COMMUNITY — *open*

| Channel | Type | Purpose |
|---------|------|---------|
| `#general` | Text | Main community chat. |
| `#introductions` | Text | New members introduce themselves. |
| `#off-topic` | Text | Memes, games, everything unrelated to the projects. |
| `Community Voice` | Voice | General hangout voice channel. |

### 🤖 KIT BOT — *open (except logs)*

| Channel | Type | Purpose |
|---------|------|---------|
| `#bot-commands` | Text | Talk to Kit (`!` commands or `@Kit` mentions). Keeps bot traffic out of other channels. |
| `#bot-help` | Text | Questions about Kit, feature requests, bug reports. |
| `#bot-logs` | Text | **Staff-only.** Kit status and diagnostic logs. |

### 🏕️ CAMPPOWERUP — *open (except announcements)*

| Channel | Type | Purpose |
|---------|------|---------|
| `#camp-announcements` | Text | **Read-only.** Project news, releases, milestones. |
| `#camp-general` | Text | General CampPowerUp discussion. |
| `#camp-dev` | Text | Development: architecture, PRs, issues, technical planning. |
| `#camp-support` | Text | Help and problem reports. |
| `#camp-feedback` | Text | Ideas and suggestions. |

### 🔧 STAFF — *staff-only*

Hidden from @everyone; visible to Admin, Moderator, Developer, and Bot.

| Channel | Type | Purpose |
|---------|------|---------|
| `#staff-chat` | Text | Private staff discussion. |
| `#mod-log` | Text | Moderation actions log. |
| `Staff Voice` | Voice | Private staff voice channel. |

## Permission Recommendations

The automation script applies these overwrites automatically; anything finer-grained should be adjusted by hand in Server Settings.

| Access preset | @everyone | Admin / Moderator | Developer | Bot |
|---------------|-----------|-------------------|-----------|-----|
| **open** | View, send, read history (from role perms) | Full | Full | Full |
| **read-only** | View + read history, ❌ send | View + send | View only | View + send (bot posts announcements/logs) |
| **staff-only** | ❌ View | View + send | View + send | View + send |

Additional recommendations (manual):

- **`#general`**: enable slowmode (5s) if the server grows.
- **`#announcements` / `#camp-announcements`**: consider converting to Announcement channels so other servers can follow them.
- **Server-wide**: set verification level to *Medium*, enable the community feature, and set `#rules` as the rules channel.
- **@everyone**: remove *Create Invite* if you want invite control; leave *Change Nickname* on.

## Onboarding Notes

### Suggested `#rules` content

> 1. **Be respectful.** No harassment, hate speech, or personal attacks.
> 2. **Keep it on-topic.** Use the right channel; take memes to #off-topic.
> 3. **No spam or self-promotion** without staff approval.
> 4. **Use #bot-commands for bot chatter** to keep other channels readable.
> 5. **Follow Discord's [Terms of Service](https://discord.com/terms) and [Community Guidelines](https://discord.com/guidelines).**
> 6. Moderators have final say. Questions? DM a @Moderator.

### Suggested `#welcome` content

> 👋 **Welcome to the Kit + CampPowerUp community!**
>
> This server is home to two projects:
> - 🤖 **Kit** — a multi-platform AI assistant bot (Slack + Discord). Try it in #bot-commands with `!help` or by mentioning @Kit.
> - 🏕️ **CampPowerUp** — check #camp-announcements for the latest news.
>
> **Getting started:**
> 1. Read the #rules
> 2. Introduce yourself in #introductions
> 3. Grab roles info in #roles-info
> 4. Say hi in #general!

### Role assignment

- Grant **Member** automatically or after onboarding (Discord's Community onboarding, or a reaction-role bot).
- Grant **Developer** manually to project contributors.
- Assign **Bot** to Kit's bot user so it can post in read-only and staff channels.

---

## Applying the template

The `cmd/server-setup` tool creates the roles, categories, and channels from `config/discord-server-template.json` in a target server. It is **idempotent**: anything that already exists (matched by name) is skipped, and nothing is ever deleted or reordered — safe to run on a live server.

### Prerequisites

- The bot is invited to the target server with **Manage Roles** and **Manage Channels** permissions (or Administrator).
- `DISCORD_BOT_TOKEN` set in the environment or `.env` (same token the bot uses — see [DISCORD_SETUP.md](DISCORD_SETUP.md)).
- The guild (server) ID — enable Developer Mode in Discord, right-click the server, *Copy Server ID*.

### Usage

```bash
# Preview what would be created (recommended first step)
go run ./cmd/server-setup -guild 123456789012345678 -dry-run

# Apply the template
go run ./cmd/server-setup -guild 123456789012345678

# Or set the guild via env and build a binary
export DISCORD_GUILD_ID=123456789012345678
go build -o server-setup ./cmd/server-setup
./server-setup
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-guild` | `$DISCORD_GUILD_ID` | Target server ID (required via flag or env). |
| `-template` | `config/discord-server-template.json` | Path to the template JSON. |
| `-dry-run` | `false` | Print planned changes without applying them. |

### Customizing for a new server

Edit `config/discord-server-template.json` (or point `-template` at a copy):

- **Roles**: `name`, `color` (decimal RGB), `hoist`, `mentionable`, and a `permissions` preset — `administrator`, `moderator`, `member`, or `bot`.
- **Categories**: `name`, an `access` preset — `open`, `read-only`, or `staff-only` — and a `channels` list.
- **Channels**: `name`, `type` (`text` or `voice`), `topic`, and an optional `access` override.

Run with `-dry-run` after editing to sanity-check the plan.
