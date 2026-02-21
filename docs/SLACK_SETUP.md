# 🚀 Slack App Setup Guide

## Quick Setup for New Slack App

### 1. Create New Slack App
1. Go to https://api.slack.com/apps
2. Click "Create New App"
3. Choose "From scratch"
4. Name: "AI Bot" (or your preferred name)
5. Select your target workspace
6. Click "Create App"

### 2. Enable Socket Mode
1. Go to "Socket Mode" in left sidebar
2. Toggle "Enable Socket Mode" to ON
3. Create an App-Level Token:
   - Token Name: "socket-token"
   - Scopes: `connections:write`
   - Click "Generate"
   - **Copy this token** (starts with xapp-) → This is your SLACK_APP_TOKEN

### 3. Configure OAuth & Permissions
1. Go to "OAuth & Permissions"
2. Scroll to "Bot Token Scopes"
3. Add these scopes:
   - `app_mentions:read`
   - `channels:history`
   - `chat:write`
   - `im:history`
   - `im:read`
   - `im:write`

### 4. Enable Event Subscriptions
1. Go to "Event Subscriptions"
2. Toggle "Enable Events" to ON
3. Under "Subscribe to bot events", add:
   - `app_mention`
   - `message.channels`
   - `message.im`

### 5. Install App to Workspace
1. Go to "Install App"
2. Click "Install to Workspace"
3. Review permissions and click "Allow"
4. **Copy the Bot User OAuth Token** (starts with xoxb-) → This is your SLACK_BOT_TOKEN

### 6. Update .env File
```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_APP_TOKEN=xapp-your-app-token-here
```

### 7. Test the Bot
1. Restart your bot application
2. Send a DM to the bot in Slack
3. Or mention the bot in a channel: @botname hello
