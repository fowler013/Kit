# Slash Commands Setup Guide

## Discord staff tickets

Discord commands use Kit's existing guild-scoped startup registration, not Slack
Socket Mode. `/help` shows approved staff `/ticket new
type:<request|feedback|bug|idea>`. The command's initial response opens a modal;
provide a required title (up to 100 characters) and details (up to 1,500).
**No medical, family, or camper personal info. Not for emergencies or safety
reports.** Follow the camp's separate safety/emergency process.

Enable only after the owner approves a destination: set optional
`KIT_TICKETS_CHANNEL_ID` to one staff-only guild text channel in the configured
`DISCORD_GUILD_ID` (or its attendance-guild fallback). Unset means disabled.
The existing `DISCORD_ATTENDANCE_STAFF_IDS` JSON allowlist authorizes command,
modal submission, and every lifecycle button; parents and campers are excluded.
Kit rechecks destination privacy before submissions and updates: `@everyone`
must explicitly be denied View Channel and all visibility grants must belong
to allowlisted staff, Kit, or the exact `CAMP_ALLOWED_ROLE` (case-insensitive).
Unknown/public audiences, permission lookup errors, wrong guilds, forums, and
thread destinations fail closed. Administrators retain Discord's inherent access.
The ticket channel need not be separately listed in `CAMP_ALLOWED_CHANNEL_IDS`.
Kit needs View Channel, Send Messages, Embed Links, Read Message History,
Create Public Threads, and Manage Threads in that channel.
**Administrator-enabled users and apps/bots bypass channel overwrites.** This
check does not exclude Appy, MEE6, Sapphire, or any other administrator-enabled
app. Before enabling intake, an operator must separately review the actual
staff/admin/app audience and permissions. Do not put sensitive data in tickets
even when the overwrite check passes.

Tickets are team-visible messages, not private adult-minor channels. Each has a
random ticket ID, category, title/details, non-pinging submitter mention, creation
time, Open status, and a discussion thread. **Claim** sets In progress with the
staff member; **Resolve** sets Resolved and archives the thread; **Reopen** sets
Open and unarchives it. Receipts and denials are ephemeral with no DMs. All posts
and edits suppress mentions and neutralize `@everyone`/`@here`. Ticket bodies
are neither logged nor sent to AI.

Status is stored in the Discord message and survives restart without a database.
The five-attempts-per-user/ten-minute rate limit resets on restart; pending forms
must be reopened. Forms expire after ten minutes and opening another replaces
the previous form. Used forms cannot repeat delivery; uncertain sends are not
retried automatically, so check the channel before submitting again. Thread
creation or archival failures explicitly report partial success for operator
follow-up. Fresh message reads and revisioned buttons reject stale actions.
Run only one active Kit instance (Discord has no atomic cross-instance edits).
Deleted messages lose their ticket state; changing the configured destination
disables old-channel controls. Parent/camper intake is a separately approved
future expansion.

Use synthetic mocked tests (`go test ./...`), not live Discord messages or
manual command registration, to validate code changes. See the README's
[staff ticket details](../README.md#staff-tickets).

## Slack setup

## Overview
This guide shows you how to add slash commands to your Kit bot, enabling users to interact with Kit using `/kit` commands.

## Step 1: Configure Slash Commands in Slack App

1. **Go to your Slack App configuration:**
   - Visit https://api.slack.com/apps
   - Select your Kit bot app

2. **Add Slash Commands:**
   - Navigate to "Slash Commands" in the left sidebar
   - Click "Create New Command"

3. **Configure the `/kit` command:**
   ```
   Command: /kit
   Request URL: (This will be handled by Socket Mode, so you can use a placeholder like https://example.com)
   Short Description: Interact with Kit AI Bot
   Usage Hint: [status|help|ask <question>]
   ```

4. **Save the command**

## Step 2: Update OAuth Scopes (if needed)

Make sure your bot has these scopes in "OAuth & Permissions":
- `commands` - to receive slash command events
- `chat:write` - to send responses
- `app_mentions:read` - for mentions
- `channels:history` - to read channel messages (if needed)
- `im:history` - to read direct messages

## Step 3: Reinstall App (if scopes changed)

If you added new scopes:
1. Click "Install to Workspace" 
2. Authorize the new permissions
3. Update your `.env` file with new tokens if needed

## Available Slash Commands

Once configured, users can use these commands:

### Basic Commands
- `/kit` - Show available commands
- `/kit status` - Check bot health and AI status
- `/kit help` - Show detailed help information
- `/kit version` - Show bot version and tech stack

### AI Interaction
- `/kit ask What is Go programming?` - Ask Kit any question
- `/kit ask How do I deploy to production?` - Get AI-powered answers
- `/kit ask Tell me a joke` - Have fun conversations

## Command Examples

```
/kit status
/kit help
/kit ask What are the benefits of microservices?
/kit ask How do I optimize database queries?
/kit ask What's the weather like? (will get a helpful response about limitations)
```

## Technical Details

### How It Works
1. User types `/kit` command in Slack
2. Slack sends the command to Kit via Socket Mode
3. Kit processes the command and subcommands
4. Kit responds with AI-generated or built-in responses
5. Response appears in the same channel

### Command Processing Flow
```
/kit ask How do I learn Go?
    ↓
Kit receives slash command event
    ↓
Parses subcommand: "ask"
    ↓
Extracts question: "How do I learn Go?"
    ↓
Sends to Gemini AI for processing
    ↓
Returns AI response to Slack channel
```

### Error Handling
- Invalid commands show helpful usage information
- AI failures fall back to basic responses
- Network issues are retried automatically

## Testing

After setup, test with:
1. `/kit` - Should show command list
2. `/kit status` - Should show bot health
3. `/kit ask Hello` - Should get AI response

## Troubleshooting

### Commands Not Working?
1. Check that slash command is created in Slack app
2. Verify Socket Mode is enabled
3. Ensure bot has `commands` scope
4. Check bot logs for error messages

### Permission Errors?
1. Reinstall app with updated scopes
2. Check OAuth permissions in Slack app settings
3. Verify bot is added to the channel/workspace

### AI Not Responding?
1. Check Gemini API key in `.env`
2. Verify API quota and billing
3. Check logs for AI-specific errors

---

## Next Steps

Once slash commands are working:
1. **Test thoroughly** - Try various commands and questions
2. **Share with team** - Let others test the new slash command interface
3. **Monitor usage** - Check logs to see which commands are popular
4. **Add more commands** - Consider adding specialized commands for your workflow

The slash commands provide a quick, discoverable way for users to interact with Kit without needing to mention the bot or send direct messages!
