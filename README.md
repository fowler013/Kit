# Slack AI Bot

A Go-based Slack bot with AI integration capabilities.

## Features

- Responds to direct messages and mentions
- Socket mode connection for real-time messaging
- Extensible AI integration (placeholder for OpenAI, Claude, etc.)
- Environment-based configuration

## Setup

### 1. Create a Slack App

1. Go to [Slack API](https://api.slack.com/apps)
2. Click "Create New App" → "From scratch"
3. Give your app a name and select your workspace
4. Go to "OAuth & Permissions" and add these Bot Token Scopes:
   - `app_mentions:read`
   - `chat:write`
   - `im:read`
   - `im:write`
5. Install the app to your workspace
6. Copy the "Bot User OAuth Token" (starts with `xoxb-`)

### 2. Enable Socket Mode

1. Go to "Socket Mode" in your app settings
2. Enable Socket Mode
3. Create an App-Level Token with `connections:write` scope
4. Copy the App-Level Token (starts with `xapp-`)

### 3. Configure Event Subscriptions

1. Go to "Event Subscriptions"
2. Enable Events
3. Subscribe to these bot events:
   - `app_mention`
   - `message.im`

### 4. Environment Setup

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Fill in your tokens in `.env`:
   ```
   SLACK_BOT_TOKEN=xoxb-your-bot-token
   SLACK_APP_TOKEN=xapp-your-app-token
   ```

### 5. Run the Bot

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Run the bot:
   ```bash
   go run main.go
   ```

## AI Integration

The bot currently includes a simple echo response. To integrate with AI services:

1. **OpenAI Integration**: Add your OpenAI API key to `.env` and implement the OpenAI client
2. **Claude Integration**: Add your Anthropic API key to `.env` and implement the Anthropic client
3. **Custom AI**: Implement your own AI service in the `generateAIResponse` function

## Development

- The bot uses Socket Mode for real-time connections
- Messages are handled in `handleEventMessage`
- AI responses are generated in `generateAIResponse`
- Environment variables are loaded from `.env` file

## Deployment

For production deployment:

1. Set environment variables in your hosting platform
2. Ensure your app has the necessary OAuth scopes
3. Consider using a process manager like systemd or supervisor
4. Set up logging and monitoring

## 📋 Project Board & Development

We use GitHub Projects for tracking development progress:

- **📊 [View Project Board](https://github.com/fowler013/Kit/projects)** - See current development status
- **🐛 [Report Issues](https://github.com/fowler013/Kit/issues/new/choose)** - Bug reports and feature requests
- **📖 [Development Guide](PROJECT_BOARD_SETUP.md)** - Complete setup instructions

### Current Development Focus

**v1.1 - Foundation** (Current)
- ✅ Basic Slack integration
- 🚧 Error handling & resilience  
- 🚧 Environment configuration
- 📋 OpenAI GPT integration

**v1.2 - AI Integration** (Next)
- 📋 Conversation memory
- 📋 Slash commands
- 📋 Structured logging

See [GITHUB_ISSUES.md](GITHUB_ISSUES.md) for complete roadmap.

## 🚀 Quick Start for Contributors

### Prerequisites
- Go 1.21+
- Slack workspace with admin access
- OpenAI API key (optional, for AI features)

### Development Setup
1. **Clone and setup:**
   ```bash
   git clone https://github.com/fowler013/Kit.git
   cd Kit
   cp .env.example .env
   # Edit .env with your tokens
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run the bot:**
   ```bash
   go run main.go
   ```

### Contributing Workflow

1. **Check the [Project Board](https://github.com/fowler013/Kit/projects)** for available issues
2. **Assign yourself** to an issue (moves to "In Progress")
3. **Create a branch:** `git checkout -b feature/123-short-description`
4. **Make changes** and test thoroughly
5. **Submit PR** linking to the issue (moves to "In Review")
6. **Merge after review** (moves to "Done")

### Branch Naming
- Features: `feature/123-openai-integration`
- Bugs: `bugfix/456-error-handling`  
- DevOps: `devops/789-docker-setup`

## 📚 Documentation

- [Project Board Setup Guide](PROJECT_BOARD_SETUP.md) - Complete development workflow
- [GitHub Issues Overview](GITHUB_ISSUES.md) - Detailed issue descriptions
- [Issue Templates](.github/ISSUE_TEMPLATE/) - For creating new issues

## 🔍 Need Help?

- 📖 Check existing [issues](https://github.com/fowler013/Kit/issues)
- 💬 Join discussions in [GitHub Discussions](https://github.com/fowler013/Kit/discussions)
- 📧 Create a new issue using our [templates](https://github.com/fowler013/Kit/issues/new/choose)
