#!/bin/bash
# Final Event Subscription Configuration Check

echo "🚨 FINAL EVENT SUBSCRIPTION DIAGNOSIS"
echo "====================================="
echo

source .env

echo "CONFIRMED ISSUE: Bot connects but receives NO message events"
echo "This means Event Subscriptions configuration has a problem."
echo

echo "🔧 REQUIRED ACTIONS TO FIX:"
echo "============================"
echo

echo "1. 🌐 Go to Slack App Management:"
echo "   https://api.slack.com/apps/A099T8XH1EG"
echo

echo "2. 📡 Check Event Subscriptions (CRITICAL):"
echo "   - Click 'Event Subscriptions' in left sidebar"
echo "   - Verify 'Enable Events' is ON"
echo "   - Check 'Subscribe to bot events' section"
echo "   - Must have EXACTLY these events:"
echo "     ✅ app_mention"
echo "     ✅ message.channels"
echo "     ✅ message.im"
echo

echo "3. 🔑 Verify OAuth Scopes:"
echo "   - Click 'OAuth & Permissions'"
echo "   - Under 'Bot Token Scopes', must have:"
echo "     ✅ app_mentions:read"
echo "     ✅ channels:history"
echo "     ✅ chat:write"
echo "     ✅ im:history"
echo "     ✅ im:read"
echo "     ✅ im:write"
echo

echo "4. 🔌 Verify Socket Mode:"
echo "   - Click 'Socket Mode'"
echo "   - Must be ENABLED"
echo "   - App-Level Token must exist"
echo

echo "5. 💾 SAVE AND REINSTALL:"
echo "   - After ANY changes, click 'Save Changes'"
echo "   - Go to 'Install App' → 'Reinstall to Workspace'"
echo "   - Accept all permissions"
echo

echo "6. 🔄 Restart Bot:"
echo "   - Kill current process: pkill -f 'go run main.go'"
echo "   - Start fresh: go run main.go > new_test.log 2>&1 &"
echo "   - Test again: send '@kit hello' in Slack"
echo

echo "🚨 MOST LIKELY ISSUES:"
echo "======================"
echo "• Event Subscriptions not properly saved"
echo "• App not fully reinstalled after configuration"
echo "• Bot events missing or incorrectly configured"
echo "• Workspace permissions blocking bot events"
echo

echo "🎯 IF STILL NOT WORKING AFTER ABOVE STEPS:"
echo "==========================================="
echo "The app may need to be DELETED and RECREATED from scratch."
echo "Sometimes Slack app configurations get into a broken state."
echo

echo "💡 IMMEDIATE TEST:"
echo "=================="
echo "After making changes and reinstalling:"
echo "1. Restart bot"
echo "2. Send '@kit test' in #all-camp-powerup"
echo "3. Look for '📨 Processing EventsAPI event...' in logs"
echo "4. If still only 'hello' events, app needs recreation"

bot_info=$(curl -s -X POST https://slack.com/api/auth.test -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json")
echo
echo "📋 Current App Info:"
echo "   App ID: A099T8XH1EG"
echo "   Workspace: $(echo "$bot_info" | jq -r '.team')"
echo "   Bot Name: $(echo "$bot_info" | jq -r '.user')"
echo "   Direct Link: https://api.slack.com/apps/A099T8XH1EG/event-subscriptions"
