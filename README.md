# Kit - Multi-Platform AI Bot 🤖

Kit is an intelligent AI bot built in Go that provides **Google Gemini AI-powered assistance** to your team on **both Slack and Discord**. The bot supports real-time communication and delivers thoughtful, contextual responses to any question or request.

## ✨ Features

- 🧠 **Google Gemini 1.5 Flash Integration**: Powered by Google's latest AI model
- 🌐 **Multi-Platform Support**: Works on both Slack and Discord simultaneously
- 🚀 **Real-time Communication**: Uses platform-native protocols for instant responses
- 💬 **Intelligent Conversations**: Context-aware responses to any question
- 📝 **Comprehensive Logging**: Detailed event logging for monitoring
- 🔧 **Easy Configuration**: Simple environment-based setup
- 🎯 **Channel Aware**: Works in public channels, private channels, and DMs
- 🛡️ **Secure**: Token-based authentication with platforms and AI services
- 🔄 **Resilient**: Automatic fallback to basic responses if AI fails
- ⚙️ **Flexible Deployment**: Run on one platform or both simultaneously

## 🚀 Quick Start

### Prerequisites
- Go 1.21+ installed
- Slack workspace with admin permissions (for Slack integration)
- Discord server with admin permissions (for Discord integration)
- Slack app configured (see [SLACK_SETUP.md](SLACK_SETUP.md))

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/fowler013/Kit.git
   cd Kit
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your platform tokens and AI API keys
   ```

4. **Build and run**
   ```bash
   go build -o kit-ai-bot
   ./kit-ai-bot
   ```

## 📖 Documentation

- **[HOW_TO_USE.md](HOW_TO_USE.md)** - Complete usage guide
- **[SLACK_SETUP.md](SLACK_SETUP.md)** - Slack app configuration
- **[DISCORD_SETUP.md](DISCORD_SETUP.md)** - Discord bot configuration
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development guidelines
- **[PROJECT_BOARD_SETUP.md](PROJECT_BOARD_SETUP.md)** - Project management

## 🔧 Configuration

Your `.env` file should contain (configure platforms as needed):
```bash
# Slack Configuration (optional)
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_APP_TOKEN=xapp-your-app-level-token-here

# Discord Configuration (optional)
DISCORD_BOT_TOKEN=your-discord-bot-token-here

# AI Configuration (required)
GEMINI_API_KEY=your-gemini-api-key-here
GEMINI_MODEL=gemini-1.5-flash
```

## 🌐 Platform Support

### Slack Integration
- Socket Mode for real-time responses
- Mentions and direct messages
- Slash commands support
- Rich message formatting

### Discord Integration  
- WebSocket connection for real-time responses
- Mentions and direct messages
- Server and DM support
- Rich embed formatting

## 💬 Usage Examples

### Slack
```
@Kit explain quantum computing in simple terms
@Kit what are best practices for team collaboration?
@Kit help me brainstorm ideas for our project
```

### Discord
```
@Kit what's the weather like today?
@Kit can you help me understand this concept?
!status
!help
```
```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_APP_TOKEN=xapp-your-app-level-token-here
GEMINI_API_KEY=your-app-level-token-here
GEMINI_MODEL=gemini-1.5-flash
```

## 💬 Usage

### In Slack Channels
```
@Kit explain quantum computing in simple terms
@Kit what are best practices for team collaboration?
@Kit help me brainstorm ideas for our project
```

### Direct Messages
```
Hello Kit! How can you help our team?
What's the best way to learn programming?
Can you review this code snippet for me?
```

## 🏗️ Project Structure

```
Kit/
├── main.go              # Main bot application
├── go.mod              # Go dependencies
├── .env                # Environment variables (private)
├── .env.example        # Environment template
├── HOW_TO_USE.md      # Usage guide
├── SLACK_SETUP.md     # Setup instructions
├── slack-ai-bot       # Compiled binary
└── test-bot.sh        # Testing script
```

## � Monitoring

Check if the bot is running:
```bash
ps aux | grep slack-ai-bot
```

View live logs:
```bash
tail -f bot.log
```

## �️ Development

### Making Changes
1. Edit `main.go`
2. Test: `go run main.go`
3. Build: `go build -o slack-ai-bot`
4. Deploy: `./slack-ai-bot`

### Adding Features
- Modify event handlers in `main.go`
- Add AI integrations (OpenAI, Claude, etc.)
- Implement custom commands
- Extend logging and monitoring

## � Troubleshooting

**Bot not responding?**
1. Check process: `ps aux | grep slack-ai-bot`
2. Check logs: `tail -f bot.log`
3. Verify tokens in `.env`
4. Ensure bot is invited to channel

**Build issues?**
1. Run: `go mod tidy`
2. Ensure only one `main.go` exists
3. Check Go version: `go version`

## 🎯 Status

✅ **Bot Successfully Running with Google Gemini AI**
- Connected to Slack via Socket Mode
- Google Gemini 1.5 Flash AI integration active
- Intelligent responses to all questions
- Receiving and responding to events
- Handling mentions and direct messages
- Comprehensive logging active
- Automatic fallback system operational

## 📞 Support

For issues and questions:
1. Check [HOW_TO_USE.md](HOW_TO_USE.md)
2. Review logs: `./test-bot.sh`
3. Verify Slack app configuration

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## � License

This project is licensed under the MIT License - see the LICENSE file for details.

---

**🎉 Congratulations! Your Kit AI Bot is ready to assist your team!** 

For detailed usage instructions, see [HOW_TO_USE.md](HOW_TO_USE.md).
