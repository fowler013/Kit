#!/bin/bash

# Slack AI Bot Testing Script
# This script helps you test your bot functionality

echo "🤖 Slack AI Bot Testing Guide"
echo "============================"
echo ""

# Check if bot is running
echo "1️⃣ Checking if bot is running..."
if pgrep -f "go run main.go" > /dev/null; then
    echo "✅ Bot is currently running!"
    echo "   Process ID: $(pgrep -f 'go run main.go')"
else
    echo "❌ Bot is not running. Start it with: go run main.go"
    exit 1
fi

echo ""
echo "2️⃣ Environment Check..."

# Check if .env file exists
if [ -f .env ]; then
    echo "✅ .env file found"
    
    # Check for required tokens (without revealing them)
    if grep -q "SLACK_BOT_TOKEN=xoxb-" .env && ! grep -q "xoxb-your-bot-token-here" .env; then
        echo "✅ SLACK_BOT_TOKEN is configured"
    else
        echo "❌ SLACK_BOT_TOKEN needs to be set in .env"
    fi
    
    if grep -q "SLACK_APP_TOKEN=xapp-" .env && ! grep -q "xapp-your-app-token-here" .env; then
        echo "✅ SLACK_APP_TOKEN is configured"
    else
        echo "❌ SLACK_APP_TOKEN needs to be set in .env"
    fi
    
    if grep -q "OPENAI_API_KEY=sk-" .env && ! grep -q "sk-your-openai-api-key-here" .env; then
        echo "✅ OPENAI_API_KEY is configured"
    else
        echo "⚠️  OPENAI_API_KEY not set (bot will use placeholder responses)"
    fi
else
    echo "❌ .env file not found. Copy from .env.example"
    exit 1
fi

echo ""
echo "3️⃣ Testing Instructions"
echo "======================="
echo ""
echo "🔍 Test Methods:"
echo ""
echo "📱 Direct Message Test:"
echo "   1. Open Slack and find your bot in Apps"
echo "   2. Send a DM: 'Hello bot!'"
echo "   3. Expected: Bot should respond with a greeting"
echo ""
echo "💬 Channel Mention Test:"
echo "   1. In any channel, type: '@YourBotName help'"
echo "   2. Expected: Bot should respond with help information"
echo ""
echo "🧠 AI Response Test:"
echo "   1. Send: 'What can you help me with?'"
echo "   2. Expected: Bot should give an AI-style response"
echo ""

echo "4️⃣ Sample Test Messages"
echo "======================"
echo ""
echo "Try these messages:"
echo "   • 'hello' → Should trigger greeting"
echo "   • 'help' → Should show help information"  
echo "   • 'How are you?' → Should get AI response"
echo "   • 'Tell me a joke' → Should get AI response"
echo ""

echo "5️⃣ Monitoring Logs"
echo "=================="
echo ""
echo "Watch the terminal where you started the bot for:"
echo "   ✅ 'WebSocket connection succeeded' - Bot connected"
echo "   ✅ Message received logs"
echo "   ✅ Response generation logs"
echo "   ❌ Any error messages"
echo ""

echo "6️⃣ Troubleshooting"
echo "=================="
echo ""
echo "If bot doesn't respond:"
echo "   1. Check bot permissions in Slack app settings"
echo "   2. Verify Socket Mode is enabled"
echo "   3. Ensure bot is added to the channel (for mentions)"
echo "   4. Check terminal for error messages"
echo ""

echo "🎉 Happy Testing!"
echo ""
echo "💡 Tip: Keep the terminal open to see real-time logs while testing"
