# Kit AI Bot - Copilot Instructions

## Build & Run Commands

```bash
# Install dependencies
go mod tidy

# Run locally (development)
go run .

# Build binary
go build -o kit-ai-bot

# Run with Docker (from repo root)
docker-compose -f config/docker-compose.yml up -d

# Run tests
go test ./...                          # All tests
go test -run TestSpecificFunction      # Single test
go test -race -cover ./...             # With race detection & coverage

# Linting & formatting
go vet ./...
gofmt -s -w .
```

## Project Structure

```
Kit/
├── *.go                    # Go source (main, gemini, claude, discord)
├── docs/                   # Documentation (SLACK_SETUP.md, HOW_TO_USE.md, etc.)
│   └── development/        # Dev docs (DEVELOPMENT.md, IMPROVEMENTS.md)
├── scripts/                # Bash + Python utilities
│   ├── setup/              # Setup scripts + env_validator.py
│   ├── debug/              # Debug scripts + log_analyzer.py
│   └── test/               # Test scripts + bot_monitor.py
└── config/                 # Config files (docker-compose.yml, redis.conf)
```

## Python Scripts

The `scripts/` directory includes Python utilities alongside bash scripts:

| Script | Location | Purpose | Status |
|--------|----------|---------|--------|
| `bot_monitor.py` | scripts/test/ | Monitor bot health | Complete (reference example) |
| `env_validator.py` | scripts/setup/ | Validate .env config | Template with TODOs |
| `log_analyzer.py` | scripts/debug/ | Analyze bot logs | Template with TODOs |

**When writing Python code**: Follow `docs/development/PYTHON_GUIDELINES.md` for conventions.

**When writing Go code**: Follow `docs/development/GO_GUIDELINES.md` for conventions.

## Architecture

Kit is a multi-platform AI chatbot written in Go that connects to Slack and Discord simultaneously. It uses Google Gemini as the primary AI backend with Claude as a fallback.

### Core Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, Slack Socket Mode client, event handlers, slash commands |
| `gemini.go` | Google Gemini AI client wrapper with system prompt configuration |
| `claude.go` | Anthropic Claude AI client wrapper (fallback) |
| `discord.go` | Discord bot using discordgo library, message handlers |

### Event Flow

1. **Slack**: Socket Mode → `handleEvents()` → `handleCallbackEvent()` → AI response
2. **Discord**: WebSocket → `onMessageCreate()` → AI response
3. Both platforms share AI clients (`globalGeminiClient`, `globalClaudeClient`)

### AI Client Pattern

All AI clients implement the same informal interface:
- `GenerateResponse(message string) (string, error)`
- Return empty string (not error) to trigger fallback to next provider

## Conventions

- **Global state**: AI clients stored in package-level vars (`globalGeminiClient`, `globalClaudeClient`, `globalBot`)
- **Logging**: Use emoji prefixes for log categories (🔵 init, ✅ success, ❌ error, 📤 outbound, 📥 inbound)
- **Response fallback chain**: Gemini → Claude → `generateBasicResponse()` hardcoded responses
- **Message formatting**: Use Slack/Discord markdown with emoji headers for bot responses
- **Command prefixes**: Slack uses `/kit`, Discord uses `!` prefix for commands
- **Retry logic**: `sendMessage()` retries up to 3 times with exponential backoff

## Environment Configuration

Required in `.env`:
```
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
DISCORD_BOT_TOKEN=...
GEMINI_API_KEY=...
```

Optional:
```
GEMINI_MODEL=gemini-1.5-flash    # default
CLAUDE_API_KEY=...
CLAUDE_MODEL=claude-3-sonnet-20240229  # default
```

## Related Academic Context (from Scince repository)

This bot is part of a broader learning ecosystem. Related course concepts that may inform feature development:

- **CYBV 326 (Networks)**: Network analysis features, protocol understanding for Discord/Slack WebSocket connections
- **CYBV 381 (Incident Response)**: Incident tracking features, security logging, response workflows
- **CYBV 302 (Linux Security)**: Deployment security, container hardening, automation scripts
- **APCV 360 (Databases)**: Potential conversation memory storage, Redis integration patterns
- **Python foundations**: The `Scince/Scince/Computer_Science/Applyd` project shares security-first design goals and could integrate with Kit for cross-platform features

When adding features, consider how they might tie into these academic domains for portfolio value.
