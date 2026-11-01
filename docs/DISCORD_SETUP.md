# Discord Bot Setup Guide

## 🚀 Setting Up Kit for Discord

Kit now supports **both Slack and Discord** simultaneously! You can run Kit on one or both platforms.

## 📋 Prerequisites

- Discord account
- Server admin permissions (to invite the bot)
- Discord Developer Portal access

## 🔧 Step-by-Step Setup

### 1. Create Discord Application

1. **Go to Discord Developer Portal:**
   - Visit: https://discord.com/developers/applications
   - Click "New Application"
   - Name: "Kit AI Bot" (or your preferred name)
   - Click "Create"

### 2. Configure Bot Settings

1. **Go to Bot section:**
   - In left sidebar, click "Bot"
   - Click "Add Bot" → "Yes, do it!"

2. **Configure Bot:**
   - **Username:** Kit (or your preference)
   - **Public Bot:** OFF (recommended for private use)
   - **Requires OAuth2 Code Grant:** OFF
   - **Privileged Gateway Intents:**
     - ✅ **Message Content Intent** (REQUIRED)
     - Leave **Server Members Intent** and **Presence Intent** disabled; Kit does not request them.

3. **Get Bot Token:**
   - Click "Reset Token" → "Yes, do it!"
   - **Copy the token** → This is your `DISCORD_BOT_TOKEN`
   - ⚠️ **Keep this secret!** Never share it publicly

### 3. Set Bot Permissions

1. **Go to OAuth2 → URL Generator:**
   - **Scopes:** Check "bot"
   - **Bot Permissions:** Check these:
     - ✅ **Send Messages**
     - ✅ **View Channel**
     - **Read Message History** is optional for viewing context.
     - No Administrator, Manage Roles/Channels, slash-command, attachment, or reaction permissions are needed for the running bot.

2. **Generate Invite URL:**
   - Copy the generated URL at the bottom
   - Save this for inviting the bot to servers

### 4. Configure Environment

1. **Add Discord token to .env:**
   ```bash
   # Add this line to your .env file
   DISCORD_BOT_TOKEN=your-discord-bot-token-here
   ```

2. **Example .env configuration:**
   ```bash
   # Slack Configuration (optional)
   SLACK_BOT_TOKEN=
   SLACK_APP_TOKEN=
   
   # Discord Configuration
   DISCORD_BOT_TOKEN=your-discord-bot-token-here
   
   # AI Configuration (optional; leave empty for basic mode)
   GEMINI_API_KEY=
   GEMINI_MODEL=
   ```

### 5. Invite Bot to Server

1. **Use the invite URL** you generated earlier
2. **Select your server** from the dropdown
3. **Review permissions** and click "Authorize"
4. **Complete captcha** if prompted

## 🎮 Using Kit on Discord

Built-in commands work in server channels without mentioning Kit. Ordinary
conversation still requires a mention (or a DM). For camp data, follow the
[private-channel and authorization requirements](DEPLOYMENT.md#1-configure-discord).

### Basic Usage

**Direct Messages:**
```
Hello Kit!
What can you help me with?
Explain quantum computing
```

**Server Mentions:**
```
@Kit what's the weather like?
@Kit help me with this code
@Kit tell me a joke
```

### Special Commands

**Status Check:**
```
!status
!health
```

**Help:**
```
!help
!commands
```

**Version Info:**
```
!version
```

**AI Questions:**
```
@Kit explain machine learning
@Kit what's the difference between Python and JavaScript?
@Kit help me debug this code
```

## 🚀 Running Kit with Discord

### Option 1: Discord Only
```bash
# Set only Discord token in .env
DISCORD_BOT_TOKEN=your-token-here
# Comment out or remove Slack tokens

go run .
```

### Option 2: Both Platforms
```bash
# Set both Slack and Discord tokens in .env
SLACK_BOT_TOKEN=your-slack-token
SLACK_APP_TOKEN=your-slack-app-token
DISCORD_BOT_TOKEN=your-discord-token

go run .
```

### Option 3: Slack Only (existing setup)
```bash
# Set only Slack tokens, no Discord token
SLACK_BOT_TOKEN=your-slack-token
SLACK_APP_TOKEN=your-slack-app-token

go run .
```

## 📊 Expected Log Output

```
🚀 Starting Kit AI Bot (Multi-Platform)...
🧠 Gemini AI initialized with model: gemini-1.5-flash
🔵 Initializing Discord integration...
🔑 Discord token loaded
✅ Discord gateway connection opened
🔵 Discord bot logged in as: Kit#1234
```

## 🔧 Troubleshooting

### Bot Not Responding?
1. **Check token:** Verify Discord token is correct in .env
2. **Check permissions:** Ensure bot has "Send Messages" permission
3. **Check intents:** Verify "Message Content Intent" is enabled
4. **Check logs:** Look for connection or authentication errors

### Permission Errors?
1. **Re-invite bot** with updated permissions
2. **Check server roles** - bot needs appropriate role hierarchy
3. **Verify channel permissions** for the specific channel

### Bot Offline?
1. **Check Developer Portal:** Verify bot status
2. **Regenerate token** if compromised
3. **Check rate limits** in console

## 🌟 Discord vs Slack Features

| Feature | Slack | Discord |
|---------|-------|---------|
| Mentions | @Kit | @Kit |
| Direct Messages | ✅ | ✅ |
| AI Responses | ✅ | ✅ |
| Status Commands | `status` | `!status` |
| Help Commands | `help` | `!help` |
| Slash Commands | `/kit` | Coming soon |
| Rich Formatting | ✅ | ✅ |

## 🚀 Next Steps

For continuous hosting, restart behavior, readiness probes, and the operator
acceptance checklist, see [DEPLOYMENT.md](DEPLOYMENT.md).

1. **Test the bot** with basic commands
2. **Invite to multiple servers** if needed
3. **Share with your community**
4. **Monitor usage** and gather feedback
5. **Consider adding Discord slash commands** (future enhancement)

---

**🎉 Congratulations! Kit is now ready to assist on Discord!**

For questions or issues, check the logs and verify your configuration matches this guide.
