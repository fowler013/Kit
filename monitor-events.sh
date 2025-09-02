#!/bin/bash
# Real-time event monitoring for the NEW bot

echo "🔍 MONITORING BOT EVENTS - NEW APP"
echo "=================================="
echo "🆔 App ID: A099TLSV51S"
echo "🤖 Bot: @kit"
echo "📁 Log: fresh_bot.log"
echo
echo "👋 Go to Slack and:"
echo "   1. Add bot to channel: /invite @kit"
echo "   2. Test: @kit hello"
echo "   3. Watch for events below..."
echo
echo "📊 Live Events:"
echo "==============="

# Monitor log file for events in real-time
tail -f fresh_bot.log | while read line; do
    # Highlight important events
    if echo "$line" | grep -q "event type:"; then
        echo "🎯 $line"
    elif echo "$line" | grep -q "EventsAPI"; then
        echo "📨 $line"
    elif echo "$line" | grep -q "message.*mention"; then
        echo "💬 $line"
    elif echo "$line" | grep -q "app_mention"; then
        echo "🔔 $line"
    elif echo "$line" | grep -q "Error\|error\|ERROR"; then
        echo "❌ $line"
    elif echo "$line" | grep -q "Success\|success\|SUCCESS"; then
        echo "✅ $line"
    else
        echo "📝 $line"
    fi
done
