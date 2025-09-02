#!/bin/bash
# Quick diagnostic to see what events are actually being received

echo "🔬 DIAGNOSTIC: What events is the bot ACTUALLY receiving?"
echo "============================================================"

# Start bot with enhanced logging
echo "Starting bot with detailed event logging..."
source .env

# Kill any existing bot
pkill -f "go run main.go" || true
sleep 1

# Start bot in background with detailed logging
go run main.go > diagnostic.log 2>&1 &
BOT_PID=$!
echo "Bot started with PID: $BOT_PID"

sleep 3
echo "Bot should be connected now..."
echo

echo "📋 Current log contents:"
echo "========================"
tail -20 diagnostic.log
echo

echo "🔍 Now checking for EventsAPI events..."
echo "========================================"
echo "Monitoring for 30 seconds..."

# Monitor for events
for i in {1..30}; do
    if grep -q "EventsAPI" diagnostic.log; then
        echo "✅ Found EventsAPI events!"
        grep "EventsAPI" diagnostic.log
        break
    fi
    
    if grep -q "app_mention\|message.*event" diagnostic.log; then
        echo "✅ Found message/mention events!"
        grep -i "app_mention\|message.*event" diagnostic.log
        break
    fi
    
    echo -n "."
    sleep 1
done

echo
echo
echo "📊 FINAL DIAGNOSTIC RESULTS:"
echo "============================="

# Check what events we actually received
echo "🎯 Event types received:"
grep "Received event type:" diagnostic.log | sort | uniq -c

echo
echo "📨 EventsAPI events:"
grep -c "EventsAPI" diagnostic.log || echo "0"

echo
echo "💬 Message events:"
grep -c "message.*event\|Message event" diagnostic.log || echo "0"

echo
echo "📢 App mention events:"
grep -c "app_mention\|App mention" diagnostic.log || echo "0"

echo
echo "🔄 Connection events:"
grep -c "hello\|connecting\|connected" diagnostic.log || echo "0"

echo
echo "📋 Last 10 log lines:"
echo "===================="
tail -10 diagnostic.log

# Stop the bot
kill $BOT_PID 2>/dev/null || true

echo
echo "✅ Diagnostic complete. Check diagnostic.log for full details."
