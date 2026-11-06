# Using Discord with Kit

If it's been a while since you used Discord, the short version is:

- A Discord server is a community space.
- Channels are rooms inside that server.
- Direct Messages (DMs) are one-to-one chats.
- Mentions trigger a bot response when the bot name is tagged.
- Commands like `!help` and `!status` are text commands sent in a channel or DM.

## How Kit uses Discord

This repo treats Discord as a platform adapter, not as the AI logic itself.

- `discord.go` owns the Discord socket connection and event handling.
- The bot listens for DMs, server mentions, and its supported `!` commands.
- It ignores bot messages and only responds to valid human input.
- The shared AI/service layer decides which provider to use and handles fallback behavior.

## Typical Discord flows in this bot

### Direct message

A user sends a DM to the bot:

```text
Hello Kit
```

The bot responds directly in that DM.

### Mention in a server

A user writes:

```text
@Kit can you summarize this issue?
```

The application strips the mention and sends a generated response in the same channel.

### Built-in commands

```text
!status
!help
!version
```

These are handled in the Discord adapter before AI generation.
They work without a mention. `!myid`, `!links`, and authorized `!camp` commands
are also available; `!camp` data is only shown in DMs and the staff channels
listed in `CAMP_ALLOWED_CHANNEL_IDS`. Unrelated unmentioned conversation and
other command prefixes are ignored.

### Beginner Discord and server help

Kit is a game guide first, but parents and campers can also ask:

```text
@Kit how do I tag someone?
!kit how do I use commands in this channel?
@Kit how do I send a message to a friend?
/kit ask prompt:What can I do in this server?
```

Kit gives short steps, with desktop/mobile differences when useful. Ask about
replies, friend requests, slash commands, threads, reactions or notifications.
It cannot click buttons, override privacy settings, or inspect a friend's DMs.
When server information is not supplied, it should say it cannot verify that
server's setup rather than inventing channels or button behavior.

Operators can set `KIT_SERVER_GUIDE` to public inline text or a local UTF-8 guide
file (maximum 4,000 characters, read only at startup). Customize
[the example guide](../server_guide.example.md) with actual public channel
purposes, roles, rules and verified button behavior. No private information.
Only navigation-like questions in the configured guild receive this guide and
a bounded public channel list: up to 40 text/announcement/forum channels,
with truncated topics and public categories. `@everyone` visibility is computed
from its base role and channel overwrites; private/staff channels are excluded.
The list uses gateway state with REST fallback and a five-minute cache.
DMs and questions outside the configured guild never receive the reference.

### Private staff recaps

Approved staff use `/kit recap channel:#minecraft`, not an @mention. The command
works only in the configured guild, for IDs in `KIT_RECAP_CHANNEL_IDS`;
staff membership comes from `DISCORD_ATTENDANCE_STAFF_IDS`. Kit needs
**View Channel + Read Message History** on each allowlisted text/announcement
channel. An empty allowlist disables recaps.

The deferred reply and all subsequent chunks are **ephemeral to the requester**,
with mentions suppressed. Kit checks up to 100 recent messages within 24 hours,
skips bots/webhooks/system messages, and summarizes the oldest-first eligible
sample. Authors become stable Camper numbers; known names/nicknames, mentions,
URLs, emails and phone-like patterns are redacted; attachments become markers,
not downloaded content. Inputs are capped near 300 characters per message and
20,000 characters total. Empty history does not call AI.

Before redaction, Kit resolves unique authors and mentioned users from guild
member state, then falls back to individual REST member lookups for missing
entries. Up to 200 participants are checked (authors first), with a two-second
timeout per REST lookup and a ten-second total metadata budget within the recap
request timeout. Resolved nicknames, usernames and global names are redacted
from both the transcript and reply. Failed lookups are tolerated: known names
are still redacted, but an unavailable nickname may remain in prose.
The bot does not enable the privileged `GUILD_MEMBERS` intent, so gateway member
coverage can be incomplete; it does not rely on a full member cache or request
a member-list download. The individual REST lookup supplies missing metadata
when Discord permits it; no additional privileged intent is enabled by this feature.

**Anonymized text goes to the configured AI provider(s)**. Redaction is
best-effort: unknown names and indirect identifying prose may survive. Operators
must review channel suitability, provider retention, and community disclosure/
consent before enabling; never allowlist sensitive personal-data channels.
Kit does not store recap transcripts/summaries in chat sessions or log message
content; Discord/provider retention still applies. The brief neutral recap is a
limited sample, not the whole history or a moderation verdict. Safety/conduct
concerns are flagged generically for staff follow-up, not quoted or attributed.

## Discord mental model for contributors

### Private attendance (not enabled yet)

The separately gated `/attendance parent` and `/attendance staff` slash-command
flows use ephemeral replies only, never the AI router or public text replies.
Parents request arrival/pickup; only authorized staff confirm physical arrival
or release after manual authorized-roster verification. No photo ID is required.
See [private attendance usage and rollout gates](DISCORD_ATTENDANCE.md).
These commands must not require opening public posting in read-only parent
channels. Backend deployment, approved family linkage, separate-account checks
and explicit command registration/activation remain operator prerequisites.

When adding functionality, think in platform terms:

- `GuildID` = server ID
- `ChannelID` = room ID
- `UserID` = user identity
- `MessageCreate` = incoming message event
- mention / DM / channel response = user interaction style

## Operational notes

- Keep the AI orchestration separate from the Discord adapter.
- Keep the bot message rules stable: ignore bot messages, handle DMs, mentions, and supported commands.
- Add session behavior at the shared service boundary instead of in Discord-specific code.

## Good starter checklist

- Invite the bot to a server
- Grant the proper permissions
- Verify the bot receives DMs and mentions
- Test `!help` and `!status`
- Confirm the shared AI router remains the single place for provider routing
