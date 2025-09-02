#!/bin/bash
# Real-time test after app reinstall

echo "🧪 POST-REINSTALL TEST"
echo "======================"

# Kill any existing bot
pkill -f "go run main.go" || true
sleep 1

echo "🚀 Starting bot with real-time monitoring..."
source .env

# Start bot
go run main.go > reinstall_test.log 2>&1 &
BOT_PID=$!
echo "Bot started with PID: $BOT_PID"

sleep 3
echo "✅ Bot should be connected now"
echo

echo "📊 Connection status:"
tail -5 reinstall_test.log | grep -E "hello|connected|EventsAPI"

echo
echo "🎯 NOW GO TO SLACK AND:"
echo "1. Add bot to a channel: /invite @kit"
echo "2. Send: @kit hello"
echo "3. Watch for events below..."
echo

echo "📡 LIVE EVENT MONITORING:"
echo "========================="

# Real-time monitoring
tail -f reinstall_test.log | while read line; do
    current_time=$(date "+%H:%M:%S")
    
    if echo "$line" | grep -q "EventsAPI"; then
        echo "🎉 [$current_time] EVENTS API: $line"
    elif echo "$line" | grep -q "Message event"; then
        echo "💬 [$current_time] MESSAGE: $line"
    elif echo "$line" | grep -q "App mention"; then
        echo "📢 [$current_time] MENTION: $line"
    elif echo "$line" | grep -q "Response sent"; then
        echo "✅ [$current_time] SENT: $line"
    elif echo "$line" | grep -q "hello.*event"; then
        echo "🔄 [$current_time] CONNECTION: Connection event (normal)"
    elif echo "$line" | grep -q "Error\|error\|❌"; then
        echo "❌ [$current_time] ERROR: $line"
    fi
done
