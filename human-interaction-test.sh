#!/bin/bash
# Test with human user interaction simulation

echo "🧪 HUMAN INTERACTION TEST"
echo "========================="
echo

source .env

echo "The bot is not receiving events from its own messages (which is normal)."
echo "Let's test if it receives events from REAL user interactions:"
echo

echo "📱 MANUAL TEST REQUIRED:"
echo "========================"
echo "1. Open Slack Camp PowerUp workspace in your browser/app"
echo "2. Go to #all-camp-powerup channel"
echo "3. Type this exact message: @kit hello test"
echo "4. Press Enter to send"
echo
echo "🔍 SIMULTANEOUSLY run this command to monitor logs:"
echo "tail -f bot_fresh.log | grep --line-buffered -E '(🎯|📨|💬|EventsAPI|app_mention)'"
echo
echo "✅ WHAT YOU SHOULD SEE if Event Subscriptions are working:"
echo "• 🎯 Received event type: events_api"
echo "• 📨 Processing EventsAPI event..."
echo "• 💬 Message event - Channel: ..."
echo
echo "❌ IF YOU ONLY SEE:"
echo "• 🎯 Received event type: hello"
echo "• 🎯 Received event type: connecting"
echo "• 🎯 Received event type: connected"
echo
echo "Then Event Subscriptions are still not working despite being configured."
echo
echo "💡 If events still don't work after real user interaction,"
echo "   the issue might be:"
echo "   1. App needs to be deleted and recreated"
echo "   2. Workspace-level restriction on bots"
echo "   3. Bot not properly added to the channel"
echo
echo "🚀 Let's also check if bot is properly in the channel:"

# Check if bot is in the target channel
echo
echo "📋 Bot channel membership:"
conv_response=$(curl -s -X POST https://slack.com/api/conversations.info \
    -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"channel": "C099PCXV2SJ"}')

is_member=$(echo "$conv_response" | jq -r '.channel.is_member')
if [ "$is_member" = "true" ]; then
    echo "✅ Bot IS a member of #all-camp-powerup"
else
    echo "❌ Bot is NOT a member of #all-camp-powerup"
    echo "   Try adding the bot to the channel first!"
fi

echo
echo "⚡ NEXT STEPS:"
echo "1. Make sure bot is in #all-camp-powerup channel"
echo "2. Send '@kit hello' from a REAL user account (not the bot)"
echo "3. Watch the logs in real-time"
echo "4. If still no events, we may need to recreate the app"
