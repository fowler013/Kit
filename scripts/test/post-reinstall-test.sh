#!/bin/bash
# Test after app reinstallation

echo "🔄 POST-REINSTALL VERIFICATION"
echo "=============================="
echo

source .env

echo "1. 🤖 Bot Identity Check:"
echo "------------------------"
bot_response=$(curl -s -X POST https://slack.com/api/auth.test -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json")
echo "$bot_response" | jq '{ok: .ok, team: .team, user: .user, bot_id: .bot_id}'

bot_id=$(echo "$bot_response" | jq -r '.user_id')

echo
echo "2. 🔍 Bot User Lookup:"
echo "---------------------"
user_response=$(curl -s -X POST https://slack.com/api/users.info -H "Authorization: Bearer $SLACK_BOT_TOKEN" -H "Content-Type: application/json" -d "{\"user\": \"$bot_id\"}")
echo "$user_response" | jq '{ok: .ok, error: .error, name: .user.name, is_bot: .user.is_bot}'

echo
echo "3. 🚀 Restart Bot and Test:"
echo "--------------------------"
echo "After app reinstallation:"
echo "1. Restart bot: pkill -f 'go run main.go' && go run main.go > test.log 2>&1 &"
echo "2. Wait 5 seconds for connection"
echo "3. Send test message: @kit hello"
echo "4. Check logs: tail -f test.log | grep 'EventsAPI'"
echo
echo "✅ If reinstallation fixed it, you should see EventsAPI events!"
