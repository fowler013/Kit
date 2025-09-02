#!/bin/bash
# Complete Slack App Recreation Guide

echo "🚀 SLACK APP RECREATION - STEP BY STEP"
echo "======================================"
echo

echo "📋 PHASE 1: DELETE OLD APP"
echo "=========================="
echo "1. Go to: https://api.slack.com/apps/A099T8XH1EG"
echo "2. Settings → Basic Information"
echo "3. Scroll down → Delete App"
echo "4. Confirm deletion"
echo
echo "✅ Press Enter when old app is DELETED..."
read -r

echo
echo "📋 PHASE 2: CREATE NEW APP"
echo "=========================="
echo "1. Go to: https://api.slack.com/apps"
echo "2. Click 'Create New App'"
echo "3. Choose 'From scratch'"
echo "4. App Name: 'Kit AI Bot' (or your choice)"
echo "5. Workspace: Select 'Camp PowerUp'"
echo "6. Click 'Create App'"
echo
echo "✅ Press Enter when new app is CREATED..."
read -r

echo
echo "📋 PHASE 3: CONFIGURE SOCKET MODE"
echo "================================="
echo "1. In your new app, go to 'Socket Mode'"
echo "2. Toggle 'Enable Socket Mode' to ON"
echo "3. Generate Token:"
echo "   - Token Name: 'socket-connection'"
echo "   - Add scope: 'connections:write'"
echo "   - Click 'Generate'"
echo "4. COPY THE APP TOKEN (starts with xapp-)"
echo
echo "Enter your new App Token (xapp-...):"
read -r new_app_token

if [[ ! "$new_app_token" =~ ^xapp-.+ ]]; then
    echo "❌ Invalid app token format. Should start with 'xapp-'"
    exit 1
fi

echo
echo "📋 PHASE 4: CONFIGURE BOT PERMISSIONS"
echo "====================================="
echo "1. Go to 'OAuth & Permissions'"
echo "2. Under 'Bot Token Scopes', add these:"
echo "   ✅ app_mentions:read"
echo "   ✅ channels:history"
echo "   ✅ chat:write"
echo "   ✅ im:history"
echo "   ✅ im:read"
echo "   ✅ im:write"
echo "   ✅ users:read"
echo
echo "✅ Press Enter when Bot Token Scopes are ADDED..."
read -r

echo
echo "📋 PHASE 5: CONFIGURE EVENT SUBSCRIPTIONS"
echo "=========================================="
echo "1. Go to 'Event Subscriptions'"
echo "2. Toggle 'Enable Events' to ON"
echo "3. Under 'Subscribe to bot events', add:"
echo "   ✅ app_mention"
echo "   ✅ message.channels"
echo "   ✅ message.im"
echo "4. Click 'Save Changes'"
echo
echo "✅ Press Enter when Event Subscriptions are CONFIGURED..."
read -r

echo
echo "📋 PHASE 6: INSTALL APP TO WORKSPACE"
echo "===================================="
echo "1. Go to 'Install App'"
echo "2. Click 'Install to Workspace'"
echo "3. Review permissions and click 'Allow'"
echo "4. COPY THE BOT TOKEN (starts with xoxb-)"
echo
echo "Enter your new Bot Token (xoxb-...):"
read -r new_bot_token

if [[ ! "$new_bot_token" =~ ^xoxb-.+ ]]; then
    echo "❌ Invalid bot token format. Should start with 'xoxb-'"
    exit 1
fi

echo
echo "📋 PHASE 7: UPDATE CONFIGURATION"
echo "================================"

# Backup current .env
cp .env .env.backup.$(date +%Y%m%d_%H%M%S)
echo "📁 Backed up existing .env file"

# Create new .env with new tokens
cat > .env << EOF
# Slack AI Bot Configuration - NEW APP

# ===================
# REQUIRED SETTINGS
# ===================

# Bot Token (starts with xoxb-) - CampPowerUp Workspace - NEW APP
SLACK_BOT_TOKEN=$new_bot_token

# App Token (starts with xapp-) - CampPowerUp Workspace - NEW APP
SLACK_APP_TOKEN=$new_app_token

# ===================
# AI SERVICE SETTINGS
# ===================

# OpenAI API Key (optional - for smarter responses)
# Get from: https://platform.openai.com/api-keys
# OPENAI_API_KEY=your-openai-api-key-here

# ===================
# BOT CONFIGURATION
# ===================

# Bot display name in conversations
BOT_NAME="Kit AI Assistant"

# Enable debug mode for detailed logging
DEBUG_MODE=true

# Logging level: debug, info, warn, error
LOG_LEVEL=info

# Maximum conversation history to maintain (per user/channel)
MAX_CONVERSATION_HISTORY=10

# ===================
# OPTIONAL SETTINGS
# ===================

# Response templates
DEFAULT_GREETING="Hello! I'm Kit, your AI assistant. How can I help you today?"
HELP_MESSAGE="I'm an AI bot that can help answer questions and have conversations. Just mention me with @kit or send me a direct message!"
ERROR_MESSAGE="I encountered an error processing your request. Please try again."

# Bot behavior settings
RESPONSE_DELAY=1
MAX_MESSAGE_LENGTH=2000

# Development settings
DEVELOPMENT_MODE=true
VERBOSE_LOGGING=true
EOF

echo "✅ Updated .env file with new tokens"

echo
echo "📋 PHASE 8: TEST NEW CONFIGURATION"
echo "=================================="

echo "Testing new tokens..."
echo "Testing Bot Token..."
bot_response=$(curl -s -X POST https://slack.com/api/auth.test \
    -H "Authorization: Bearer $new_bot_token" \
    -H "Content-Type: application/json")

if echo "$bot_response" | grep -q '"ok":true'; then
    echo "✅ Bot token is valid"
    team_name=$(echo "$bot_response" | sed -n 's/.*"team":"\([^"]*\)".*/\1/p')
    bot_name=$(echo "$bot_response" | sed -n 's/.*"user":"\([^"]*\)".*/\1/p')
    bot_id=$(echo "$bot_response" | sed -n 's/.*"user_id":"\([^"]*\)".*/\1/p')
    echo "   🏢 Workspace: $team_name"
    echo "   🤖 Bot Name: $bot_name"
    echo "   🆔 Bot ID: $bot_id"
else
    echo "❌ Bot token test failed"
    echo "Response: $bot_response"
    exit 1
fi

echo
echo "Testing App Token..."
app_response=$(curl -s -X POST https://slack.com/api/apps.connections.open \
    -H "Authorization: Bearer $new_app_token" \
    -H "Content-Type: application/json")

if echo "$app_response" | grep -q '"ok":true'; then
    echo "✅ App token is valid"
    echo "   🔌 Socket Mode connection available"
else
    echo "❌ App token test failed"
    echo "Response: $app_response"
    exit 1
fi

echo
echo "🎉 SUCCESS! NEW APP IS CONFIGURED!"
echo "=================================="
echo "📋 New App Details:"
echo "   Workspace: $team_name"
echo "   Bot Name: $bot_name"
echo "   Bot ID: $bot_id"
echo
echo "🚀 NEXT STEPS:"
echo "1. Add bot to channels: In Slack, go to #all-camp-powerup"
echo "2. Type: /invite @$bot_name"
echo "3. Start bot: go run main.go > fresh_bot.log 2>&1 &"
echo "4. Test: Send '@$bot_name hello' in the channel"
echo "5. Check logs: tail -f fresh_bot.log"
echo
echo "✅ The new app should work perfectly!"
