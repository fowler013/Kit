# Kit AI Slack Bot - How to Use Guide

## 🎉 Quick Start

Your Kit AI Bot is now successfully set up with **Claude AI integration**! Here's everything you need to know to use and maintain your intelligent Slack assistant.

## 🔧 Prerequisites

- ✅ Go 1.21+ installed
- ✅ Slack workspace with admin permissions
- ✅ Kit AI Bot app installed in your Slack workspace
- ✅ Anthropic Claude API key configured

## 🚀 Running the Bot

### Option 1: Direct Run
```bash
go run main.go
```

### Option 2: Build and Run
```bash
go build -o slack-ai-bot
./slack-ai-bot
```

### Option 3: Service Management
```bash
./bot-service.sh start    # Start bot
./bot-service.sh stop     # Stop bot
./bot-service.sh restart  # Restart bot
./bot-service.sh status   # Check status
./bot-service.sh logs     # View live logs
```

## 💬 Using the Bot in Slack

### 1. **Mention the Bot** (Now with AI!)
In any channel where the bot is present:
```
@Kit what's the weather like today?
@Kit can you help me understand this concept?
@Kit explain machine learning in simple terms
@Kit how do I write better code?
```

### 2. **Direct Message** (Full AI Conversation)
Send a DM directly to the Kit bot:
```
Hello Kit! How are you today?
Can you help me plan a project?
What are some good programming practices?
Explain quantum computing to me
```

### 3. **Invite to Channels**
```
/invite @Kit
```

## 🧠 AI Features

- **Claude 3 Sonnet**: Powered by Anthropic's latest AI model
- **Intelligent Responses**: Context-aware, helpful answers
- **Professional Tone**: Optimized for workplace communication
- **Fallback System**: Basic responses if AI is unavailable
- **Conversation Memory**: Understands context within messages
- **Error Resilience**: Graceful handling of API issues

## 🔍 Bot Features

- **Smart Responses**: AI-generated, contextual answers
- **Channel Awareness**: Works in both public channels and DMs
- **Event Logging**: Detailed logging for debugging and monitoring
- **Socket Mode**: Real-time connection to Slack via WebSocket
- **Automatic Fallback**: Basic responses when AI is unavailable

## 📊 Monitoring & Logs

### Check if Bot is Running
```bash
ps aux | grep slack-ai-bot
```

### View Live Logs
```bash
tail -f bot.log
```

### Test Bot Connectivity
```bash
./test-bot.sh
```

## 🔧 Configuration

### Environment Variables (.env)
```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
```

### Bot Permissions Required
- `app_mentions:read` - To receive mentions
- `channels:read` - To read channel information
- `chat:write` - To send messages
- `im:read` - To read direct messages
- `im:write` - To send direct messages

## 🛠️ Troubleshooting

### Bot Not Responding?
1. Check if the process is running: `ps aux | grep slack-ai-bot`
2. Check logs: `tail -f bot.log`
3. Verify tokens in `.env` file
4. Ensure bot is invited to the channel

### Connection Issues?
1. Verify App-Level Token starts with `xapp-`
2. Verify Bot Token starts with `xoxb-`
3. Check Slack app permissions
4. Restart the bot: `pkill slack-ai-bot && ./slack-ai-bot &`

### Build Issues?
1. Ensure only one `main.go` file exists
2. Run: `go mod tidy`
3. Run: `go build -o slack-ai-bot`

## 📁 Project Structure

```
Kit/
├── main.go              # Main bot application
├── go.mod              # Go module dependencies
├── go.sum              # Go module checksums
├── .env                # Environment variables (keep private!)
├── .env.example        # Example environment file
├── README.md           # Project overview
├── HOW_TO_USE.md      # This guide
├── SLACK_SETUP.md     # Slack app setup instructions
├── slack-ai-bot       # Compiled binary
├── test-bot.sh        # Bot testing script
└── archive/           # Old logs and debug scripts
```

## 🔄 Development Workflow

### Making Changes
1. Edit `main.go`
2. Test: `go run main.go`
3. Build: `go build -o slack-ai-bot`
4. Deploy: `./slack-ai-bot`

### Adding Features
1. Modify event handlers in `main.go`
2. Add new dependencies: `go get package-name`
3. Update `go.mod`: `go mod tidy`
4. Test thoroughly before deploying

## 🎯 Example Interactions

### Basic Chat
```
User: @Kit hello
Bot: Hello! I'm Kit, your AI assistant. How can I help you today? 👋

User: @Kit what's the weather?
Bot: Hi there! I received your message: "what's the weather?"
     I'm Kit, your AI assistant. How can I help you today? 🤖
```

### Bot Behavior
- ✅ Responds to @mentions in channels
- ✅ Responds to direct messages
- ✅ Ignores its own messages
- ✅ Logs all events for debugging
- ❌ Ignores non-mention channel messages (by design)

## 🔐 Security Notes

- Keep `.env` file private and never commit it
- Regenerate tokens if compromised
- Use environment variables for all secrets
- Monitor logs for unusual activity

## 📞 Support

If you encounter issues:
1. Check the logs: `tail -f bot.log`
2. Run diagnostics: `./test-bot.sh`
3. Review this guide
4. Check Slack app settings in your workspace

---

## 🎊 Congratulations!

Your Kit AI Bot is successfully running and ready to assist your team in Slack! 

**Next Steps:**
- Customize the bot responses in `main.go`
- Add AI integration (OpenAI, Claude, etc.)
- Implement custom commands and features
- Set up monitoring and alerts

Happy botting! 🤖✨
