#!/bin/bash
# Direct verification of app configuration

echo "🔍 VERIFYING APP CONFIGURATION"
echo "=============================="

source .env

echo "📋 App Details:"
echo "App ID: A099TLSV51S"
echo "Bot Token: ${SLACK_BOT_TOKEN:0:20}..."
echo "App Token: ${SLACK_APP_TOKEN:0:20}..."
echo

echo "🔗 DIRECT LINKS TO FIX THE ISSUE:"
echo "================================="
echo
echo "1. 🎯 GO TO YOUR APP'S EVENT SUBSCRIPTIONS:"
echo "   https://api.slack.com/apps/A099TLSV51S/event-subscriptions"
echo
echo "2. ✅ VERIFY THESE SETTINGS:"
echo "   - Enable Events: ON"
echo "   - Bot Events must include:"
echo "     • app_mention"
echo "     • message.channels"
echo "     • message.im"
echo
echo "3. 💾 CLICK 'SAVE CHANGES'"
echo
echo "4. 🔄 REINSTALL APP:"
echo "   https://api.slack.com/apps/A099TLSV51S/install-on-team"
echo
echo "5. 🧪 TEST BY ADDING BOT TO CHANNEL AND MENTIONING IT"

echo
echo "⚠️  IF THE ABOVE LINKS DON'T WORK:"
echo "===================================="
echo "Go to: https://api.slack.com/apps"
echo "Find: 'Kit AI Bot' or similar"
echo "Click it, then follow steps above"

echo
echo "🚨 CRITICAL: Without proper Event Subscriptions,"
echo "   the bot will ONLY receive connection events!"
echo "   This explains why we only see 'hello' events."
