#!/bin/bash
# Check for common Event Subscription issues

echo "🚨 CRITICAL EVENT SUBSCRIPTION CHECKLIST"
echo "========================================"
echo
echo "Based on diagnostics, your tokens are valid and Socket Mode is working,"
echo "but events aren't reaching the bot. This indicates one of these issues:"
echo
echo "🔧 ISSUE #1: Request URL in Event Subscriptions"
echo "-----------------------------------------------"
echo "❌ PROBLEM: If you have a Request URL set in Event Subscriptions,"
echo "   Slack will send events to that URL instead of Socket Mode."
echo
echo "✅ SOLUTION:"
echo "   1. Go to: https://api.slack.com/apps/A099T8XH1EG/event-subscriptions"
echo "   2. Check 'Request URL' field"
echo "   3. It should be EMPTY (since you're using Socket Mode)"
echo "   4. If there's a URL, DELETE it"
echo "   5. Save changes and reinstall the app"
echo
echo "🔧 ISSUE #2: Event Subscription Workspace Mismatch"
echo "--------------------------------------------------"
echo "❌ PROBLEM: Event Subscriptions configured for wrong workspace"
echo
echo "✅ SOLUTION:"
echo "   1. Make sure you're configuring the 'Kit' app"
echo "   2. Verify it's the one for 'Camp PowerUp' workspace"
echo "   3. Check the app URL shows your workspace name"
echo
echo "🔧 ISSUE #3: App Permissions Need Refresh"
echo "-----------------------------------------"
echo "❌ PROBLEM: Old cached permissions"
echo
echo "✅ SOLUTION:"
echo "   1. Go to: https://api.slack.com/apps/A099T8XH1EG/oauth"
echo "   2. Click 'Reinstall to Workspace'"
echo "   3. Accept all permissions"
echo
echo "🔧 ISSUE #4: Bot Not Properly Invited to Workspace"
echo "--------------------------------------------------"
echo "❌ PROBLEM: Bot exists but isn't fully active"
echo
echo "✅ SOLUTION:"
echo "   1. In Slack, go to Apps section"
echo "   2. Find 'Kit' bot"
echo "   3. Click 'Add to Channel' for any channel"
echo "   4. Try sending @kit hello in that channel"
echo
echo "⚡ IMMEDIATE ACTION:"
echo "==================="
echo "1. Check Event Subscriptions Request URL (most likely issue)"
echo "2. If Request URL is empty, check bot event subscriptions list"
echo "3. Ensure these EXACT events are listed:"
echo "   • app_mention"
echo "   • message.channels"
echo "   • message.im"
echo
echo "🧪 AFTER FIXING:"
echo "=================="
echo "1. Restart bot: ./restart-bot.sh"
echo "2. Watch logs: tail -f bot.log"
echo "3. Send test message: @kit hello"
echo "4. Look for: '📨 Processing EventsAPI event...'"
echo
echo "💡 The #1 issue is usually Request URL being set when using Socket Mode!"
