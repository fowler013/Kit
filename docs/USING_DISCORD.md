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
- The bot listens for DMs and server mentions.
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

## Discord mental model for contributors

When adding functionality, think in platform terms:

- `GuildID` = server ID
- `ChannelID` = room ID
- `UserID` = user identity
- `MessageCreate` = incoming message event
- mention / DM / channel response = user interaction style

## Operational notes

- Keep the AI orchestration separate from the Discord adapter.
- Keep the bot message rules stable: ignore bot messages, handle DMs and mentions, and preserve user-facing commands.
- Add session behavior at the shared service boundary instead of in Discord-specific code.

## Good starter checklist

- Invite the bot to a server
- Grant the proper permissions
- Verify the bot receives DMs and mentions
- Test `!help` and `!status`
- Confirm the shared AI router remains the single place for provider routing
