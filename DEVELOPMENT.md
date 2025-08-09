# Development Guide

## 🚀 Quick Start

### Local Development

1. **Clone and setup:**
   ```bash
   git clone https://github.com/fowler013/Kit.git
   cd Kit
   cp .env.example .env
   # Edit .env with your tokens
   ```

2. **Run locally:**
   ```bash
   go mod tidy
   go run main.go
   ```

3. **Run with Docker:**
   ```bash
   docker-compose up -d
   ```

### Development Workflow

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/123-short-description
   ```

2. **Make changes and test:**
   ```bash
   go test ./...
   go vet ./...
   gofmt -s -w .
   ```

3. **Commit and push:**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   git push origin feature/123-short-description
   ```

4. **Create Pull Request:**
   - Link to the GitHub issue
   - CI will automatically run tests
   - Request review from maintainers

## 🧪 Testing

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run specific test
go test -run TestSpecificFunction
```

### Test Structure
```
├── main_test.go           # Main package tests
├── handlers/
│   └── handlers_test.go   # Handler tests
└── testdata/             # Test fixtures
    ├── messages.json
    └── responses.json
```

## 🐳 Docker Development

### Building Images
```bash
# Build the application image
docker build -t slack-ai-bot .

# Build and run with compose
docker-compose up --build
```

### Container Commands
```bash
# View logs
docker-compose logs -f slack-ai-bot

# Execute commands in container
docker-compose exec slack-ai-bot sh

# Access Redis CLI
docker-compose exec redis redis-cli

# View Redis data (with redis-commander)
docker-compose --profile debug up -d
# Open http://localhost:8081
```

## 🔧 Configuration

### Environment Variables

#### Required
- `SLACK_BOT_TOKEN` - Bot User OAuth Token
- `SLACK_APP_TOKEN` - App-Level Token

#### AI Services (choose one)
- `OPENAI_API_KEY` - OpenAI API key
- `ANTHROPIC_API_KEY` - Anthropic Claude API key

#### Optional
- `DEBUG_MODE` - Enable debug logging (default: false)
- `LOG_LEVEL` - Logging level (debug, info, warn, error)
- `REDIS_URL` - Redis connection string
- `HTTP_PORT` - Health check port (default: 8080)

### Configuration Files
- `.env` - Local environment variables
- `redis.conf` - Redis configuration
- `docker-compose.yml` - Container orchestration
- `.codeclimate.yml` - Code quality rules

## 📦 Dependencies

### Core Dependencies
```go
github.com/slack-go/slack        // Slack API client
github.com/joho/godotenv        // Environment variable loading
```

### AI Integrations
```go
github.com/sashabaranov/go-openai  // OpenAI integration
// github.com/anthropic/claude-go   // Claude integration (when available)
```

### Optional Dependencies
```go
github.com/go-redis/redis/v8     // Redis client (for conversation memory)
github.com/gorilla/mux          // HTTP router (for health checks)
github.com/sirupsen/logrus      // Structured logging
```

## 🏗️ Architecture

### Project Structure
```
├── main.go                 # Application entry point
├── handlers/               # Message handlers
│   ├── events.go          # Slack event handling
│   ├── commands.go        # Slash command handling
│   └── interactive.go     # Interactive component handling
├── ai/                    # AI service integrations
│   ├── openai.go         # OpenAI client
│   ├── claude.go         # Claude client
│   └── interface.go      # AI service interface
├── memory/                # Conversation memory
│   ├── redis.go          # Redis implementation
│   └── interface.go      # Memory interface
├── config/                # Configuration
│   └── config.go         # Configuration loading
└── health/                # Health checks
    └── health.go         # Health check handlers
```

### Key Components

#### 1. Event Handler
- Processes Slack events (messages, mentions)
- Routes to appropriate handlers
- Manages context and user sessions

#### 2. AI Service Interface
```go
type AIService interface {
    GenerateResponse(ctx context.Context, messages []Message) (string, error)
    GetModel() string
    ValidateConfig() error
}
```

#### 3. Memory Interface
```go
type Memory interface {
    Store(userID string, messages []Message) error
    Retrieve(userID string, limit int) ([]Message, error)
    Clear(userID string) error
}
```

## 🚦 CI/CD Pipeline

### GitHub Actions Workflows

#### CI Pipeline (`.github/workflows/ci.yml`)
- **Triggers:** Push to main/develop, PRs to main
- **Steps:**
  1. Go vet and formatting checks
  2. Unit tests with race detection
  3. Security scanning (Gosec, Trivy)
  4. Build verification
  5. Slack notifications

#### Deploy Pipeline (`.github/workflows/deploy.yml`)
- **Triggers:** Git tags (v*), manual dispatch
- **Steps:**
  1. Build and push Docker image
  2. Deploy to staging (automatic)
  3. Deploy to production (manual approval)
  4. Slack notifications

### Branch Protection Rules
Configure these rules for the `main` branch:
- Require PR reviews (1 reviewer minimum)
- Require status checks (CI pipeline)
- Require up-to-date branches
- Include administrators in restrictions

## 🔒 Security

### Best Practices
- All secrets in environment variables
- Non-root Docker containers
- Security scanning in CI pipeline
- Dependency vulnerability checks
- Input validation and sanitization

### Required Secrets
- `SLACK_BOT_TOKEN` - Repository secret
- `SLACK_APP_TOKEN` - Repository secret  
- `SLACK_WEBHOOK_URL` - For CI notifications
- `OPENAI_API_KEY` - For AI integration

## 📊 Monitoring

### Health Checks
```bash
# Local health check
curl http://localhost:8080/health

# Docker health check
docker-compose ps
```

### Logging
- Structured JSON logs in production
- Configurable log levels
- Request correlation IDs
- Error tracking and alerting

### Metrics (Future)
- Response times
- AI service usage
- User engagement
- Error rates

## 🤝 Contributing

1. Check the [Project Board](https://github.com/fowler013/Kit/projects) for available issues
2. Follow the development workflow above
3. Ensure all tests pass and code is formatted
4. Submit PR with clear description and link to issue
5. Request review from maintainers

## 🆘 Troubleshooting

### Common Issues

**Bot not responding:**
- Check Slack tokens are valid
- Verify Socket Mode is enabled
- Check bot permissions in Slack

**AI integration failing:**
- Verify API keys are set correctly
- Check API rate limits
- Review AI service status

**Docker issues:**
- Ensure Docker daemon is running
- Check environment variables in `.env`
- Review container logs

### Debug Commands
```bash
# Enable debug mode
export DEBUG_MODE=true
go run main.go

# Check environment
env | grep SLACK
env | grep OPENAI

# Test AI integration
go run -tags debug main.go --test-ai
```
