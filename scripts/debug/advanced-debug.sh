#!/bin/bash
# Test if the issue is with the bot not properly handling events

echo "🔬 ADVANCED EVENT DEBUGGING"
echo "=========================="
echo

source .env

echo "1. 🎯 Testing Direct API Call vs Socket Mode"
echo "--------------------------------------------"

# Get bot info
bot_info=$(curl -s -X POST https://slack.com/api/auth.test -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json")
bot_id=$(echo "$bot_info" | jq -r '.user_id')

echo "Bot ID: $bot_id"

# Test sending a message and check if we can see it via API
echo
echo "2. 📝 Sending test message and checking via API..."
channel_id="C099PCXV2SJ"  # all-camp-powerup channel

# Send a test message
test_message="🧪 Debug test $(date +%H:%M:%S) - checking event flow"
send_response=$(curl -s -X POST https://slack.com/api/chat.postMessage \
    -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"channel\":\"$channel_id\",\"text\":\"$test_message\"}")

if echo "$send_response" | jq -r '.ok' | grep -q "true"; then
    message_ts=$(echo "$send_response" | jq -r '.ts')
    echo "✅ Message sent successfully"
    echo "   Timestamp: $message_ts"
    
    # Now check if we can read the message back
    echo
    echo "3. 📖 Reading messages from channel via API..."
    read_response=$(curl -s -X POST https://slack.com/api/conversations.history \
        -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"channel\":\"$channel_id\",\"limit\":3}")
    
    if echo "$read_response" | jq -r '.ok' | grep -q "true"; then
        echo "✅ Can read channel messages"
        echo "   Recent messages:"
        echo "$read_response" | jq -r '.messages[] | "   - " + .text' | head -3
    else
        echo "❌ Cannot read channel messages"
        echo "   Error: $(echo "$read_response" | jq -r '.error')"
    fi
else
    echo "❌ Failed to send message"
    echo "   Error: $(echo "$send_response" | jq -r '.error')"
fi

echo
echo "4. 🔌 Socket Mode Connection Deep Check"
echo "--------------------------------------"

# Test Socket Mode connection with more detail
socket_response=$(curl -s -X POST https://slack.com/api/apps.connections.open \
    -H "Authorization: Bearer $SLACK_APP_TOKEN" \
    -H "Content-Type: application/json")

echo "Socket Mode Response:"
echo "$socket_response" | jq '.'

echo
echo "5. 🧬 App Token Validation"
echo "--------------------------"

# Validate app token
if [[ "$SLACK_APP_TOKEN" == xapp-* ]]; then
    echo "✅ App token format is correct (xapp-*)"
    
    # Check if the app token has the right scope
    app_info=$(curl -s -X POST https://slack.com/api/auth.test \
        -H "Authorization: Bearer $SLACK_APP_TOKEN" \
        -H "Content-Type: application/json")
    
    if echo "$app_info" | jq -r '.ok' | grep -q "true"; then
        echo "✅ App token is valid"
        echo "   Team: $(echo "$app_info" | jq -r '.team')"
    else
        echo "❌ App token validation failed"
        echo "   Error: $(echo "$app_info" | jq -r '.error')"
    fi
else
    echo "❌ App token format is incorrect"
fi

echo
echo "6. 🔍 Real-time Log Analysis"
echo "----------------------------"

# Check the last few log entries for any hidden errors
echo "Recent log entries (looking for errors):"
tail -10 bot.log | grep -E "(error|Error|ERROR|failed|Failed)" || echo "No obvious errors in recent logs"

echo
echo "7. 🎪 Final Test - Manual Event Trigger"
echo "---------------------------------------"

echo "Now let's see what the bot receives when we trigger events manually..."
echo "The bot should be listening. Check your bot logs now:"
echo "   tail -f bot.log"
echo
echo "Go to Slack and:"
echo "1. Send a DM to the 'kit' bot: 'hello'"
echo "2. In #all-camp-powerup channel, type: '@kit test'"
echo
echo "🔍 What to look for in logs:"
echo "   ✅ Should see: '📨 Processing EventsAPI event...'"
echo "   ✅ Should see: '💬 Message event - Channel: ...'"
echo "   ❌ If only 'hello' events, Event Subscriptions not working"
