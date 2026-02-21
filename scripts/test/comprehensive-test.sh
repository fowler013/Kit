#!/bin/bash
# Comprehensive Event Subscription Test

echo "🔬 COMPREHENSIVE EVENT SUBSCRIPTION TEST"
echo "========================================"
echo

source .env

echo "1. 🏠 Current Bot Status:"
echo "------------------------"
bot_info=$(curl -s -X POST https://slack.com/api/auth.test -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json")
team_name=$(echo "$bot_info" | jq -r '.team')
bot_name=$(echo "$bot_info" | jq -r '.user')
bot_id=$(echo "$bot_info" | jq -r '.user_id')

echo "   Workspace: $team_name"
echo "   Bot Name: $bot_name"
echo "   Bot ID: $bot_id"

echo
echo "2. 🔌 Socket Mode Connection:"
echo "----------------------------"
socket_test=$(curl -s -X POST https://slack.com/api/apps.connections.open -H "Authorization: Bearer $SLACK_APP_TOKEN" -H "Content-Type: application/json")
if echo "$socket_test" | jq -r '.ok' | grep -q "true"; then
    echo "   ✅ Socket Mode: Working"
else
    echo "   ❌ Socket Mode: Failed"
    echo "   Response: $socket_test"
fi

echo
echo "3. 📋 Current Log Status:"
echo "------------------------"
if ps aux | grep "go run main.go" | grep -v grep > /dev/null; then
    echo "   ✅ Bot Process: Running"
    echo "   📊 Recent events in logs:"
    
    # Check what types of events we're receiving
    event_types=$(tail -50 bot.log | grep "🎯 Received event type:" | tail -5)
    if [ -n "$event_types" ]; then
        echo "$event_types" | sed 's/^/      /'
    else
        echo "      No recent events found"
    fi
else
    echo "   ❌ Bot Process: Not Running"
fi

echo
echo "4. 🧪 Live Event Test:"
echo "----------------------"
echo "   Sending test message to trigger events..."

# Get available channels
channels=$(curl -s -X POST https://slack.com/api/conversations.list -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json" -d '{"types": "public_channel", "limit": 5}')
channel_id=$(echo "$channels" | jq -r '.channels[0].id')
channel_name=$(echo "$channels" | jq -r '.channels[0].name')

if [ "$channel_id" != "null" ]; then
    echo "   Target Channel: #$channel_name ($channel_id)"
    
    # Send a message that should trigger events
    message_response=$(curl -s -X POST https://slack.com/api/chat.postMessage \
        -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
        -H "Content-Type: application/json; charset=utf-8" \
        -d "{\"channel\":\"$channel_id\",\"text\":\"<@$bot_id> Event test $(date +%H:%M:%S) - checking Event Subscriptions\"}")
    
    if echo "$message_response" | jq -r '.ok' | grep -q "true"; then
        echo "   ✅ Message sent successfully"
        
        echo
        echo "5. 🔍 Monitoring for Events (10 seconds):"
        echo "-----------------------------------------"
        echo "   Watching logs for EventsAPI activity..."
        
        # Monitor logs for 10 seconds
        timeout 10 tail -f bot.log | while read line; do
            if echo "$line" | grep -q "EventsAPI\|app_mention\|Message event"; then
                echo "   🎉 EVENT DETECTED: $line"
                break
            elif echo "$line" | grep -q "🎯 Received event type:"; then
                echo "   📡 Event received: $(echo "$line" | sed 's/.*🎯 Received event type: //')"
            fi
        done &
        monitor_pid=$!
        
        sleep 11
        kill $monitor_pid 2>/dev/null
        
        echo
        echo "6. 📊 Final Analysis:"
        echo "--------------------"
        recent_events=$(tail -20 bot.log | grep "🎯 Received event type:" | tail -3)
        if echo "$recent_events" | grep -q "events_api\|EventsAPI"; then
            echo "   ✅ SUCCESS: Bot is receiving EventsAPI events!"
        elif echo "$recent_events" | grep -q "hello\|connecting\|connected"; then
            echo "   ⚠️  ISSUE: Bot only receiving connection events"
            echo "   🔧 Action needed: Check Event Subscriptions configuration"
        else
            echo "   ❓ Status unclear - check logs manually"
        fi
        
    else
        echo "   ❌ Failed to send test message"
        echo "   Response: $message_response"
    fi
else
    echo "   ❌ No accessible channels found"
fi

echo
echo "🎯 DEBUGGING CHECKLIST:"
echo "======================="
echo "□ Event Subscriptions enabled in Slack app"
echo "□ Bot events added: app_mention, message.channels, message.im"
echo "□ App reinstalled after changes"
echo "□ Bot has proper OAuth scopes"
echo "□ Socket Mode enabled"
echo
echo "📱 Try this in Slack:"
echo "1. Go to Camp PowerUp workspace"
echo "2. Find a channel where the bot is present"
echo "3. Type: @kit hello"
echo "4. Watch the logs: tail -f bot.log"
