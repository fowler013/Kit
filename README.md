# Kit - Multi-Platform AI Bot 🤖

Kit is a Go application that connects to Discord and/or Slack. Run it continuously
as a Gateway bot, with optional AI providers and authorized CampPowerUp lookups.
For hosting, server permissions, and go-live checks, start with
**[Deploy Kit to Discord](docs/DEPLOYMENT.md)**.

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
- Go 1.25 installed (see the minimum/toolchain versions in `go.mod`)
- Slack workspace with admin permissions (for Slack integration)
- Discord server with admin permissions (for Discord integration)
- Slack app configured only if using Slack (see [SLACK_SETUP.md](docs/SLACK_SETUP.md))

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/fowler013/Kit.git
   cd Kit
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Set DISCORD_BOT_TOKEN; leave unused Slack and AI settings empty
   ```

4. **Build and run**
   ```bash
   go build -o kit-ai-bot
   ./kit-ai-bot
   ```

## 📖 Documentation

- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** - Continuous hosting, health checks, camp connection, and go-live checklist
- **[HOW_TO_USE.md](docs/HOW_TO_USE.md)** - Complete usage guide
- **[USING_DISCORD.md](docs/USING_DISCORD.md)** - Beginner-friendly Discord interactions guide
- **[SLACK_SETUP.md](docs/SLACK_SETUP.md)** - Slack app configuration
- **[DISCORD_SETUP.md](docs/DISCORD_SETUP.md)** - Discord bot configuration
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development guidelines
- **[PROJECT_BOARD_SETUP.md](PROJECT_BOARD_SETUP.md)** - Project management

## 🔧 Configuration

Your `.env` file should contain (configure platforms as needed):
```bash
# Slack Configuration (optional; set both or leave both empty)
SLACK_BOT_TOKEN=
SLACK_APP_TOKEN=

# Discord Configuration (optional)
DISCORD_BOT_TOKEN=your-discord-bot-token-here

# AI Configuration (optional; select an available model in your provider console)
GEMINI_API_KEY=
GEMINI_MODEL=
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
- Built-in `!` commands in server channels without a mention
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

## 🧩 Platform Skills and Structure

The app is intentionally split into platform adapters and shared service logic so Discord and Slack can evolve independently without duplicating the AI routing layer.

- `Slack skill`: event routing, mentions, direct messages, slash commands
- `Discord skill`: DM handling, mentions, server commands, bot status triggers
- `Shared AI boundary`: provider orchestration, session memory, fallback logic
- `MCP-ready request layer`: a thin adapter boundary for future external tool ingestion

## 🏗️ Project Structure

```
Kit/
├── main.go                 # Bootstraps Slack/Discord and app lifecycle
├── service.go              # Shared AI router, chat request boundary, session memory
├── gemini.go               # Google Gemini AI client
├── claude.go               # Anthropic Claude AI client
├── discord.go              # Discord adapter and message handling
├── go.mod                  # Go dependencies
├── Dockerfile              # Container build
├── docs/                   # Documentation
│   ├── USING_DISCORD.md    # Beginner-friendly Discord usage guide
│   ├── SLACK_SETUP.md      # Slack app configuration
│   ├── DISCORD_SETUP.md    # Discord bot configuration
│   ├── HOW_TO_USE.md       # Usage guide
│   └── development/        # Dev docs (DEVELOPMENT.md, etc.)
├── scripts/                # Shell scripts
│   ├── setup/              # Setup & configuration scripts
│   ├── debug/              # Debugging scripts
│   └── test/               # Test scripts
├── config/                 # Configuration files
│   ├── docker-compose.yml  # Docker orchestration
│   └── redis.conf          # Redis configuration
├── .env.example            # Example environment configuration
└── README.md               # Project overview and usage
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
2. Test: `go test ./...`
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

Deployment status depends on your configured credentials and hosting service.
This repository does not establish that a live bot is online. Follow the
[deployment acceptance checks](docs/DEPLOYMENT.md#5-go-live-acceptance-checks).
`/healthz` reports HTTP liveness; `/readyz` reports chat-gateway readiness.

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
