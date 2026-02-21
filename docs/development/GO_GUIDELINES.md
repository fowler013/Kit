# Go Guidelines for Kit

This document defines Go conventions for the Kit project. Follow these patterns for consistency across all Go code.

## Quick Reference

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/external/package"
)

// TypeName describes what this type represents
type TypeName struct {
    fieldName string
    clientPtr *SomeClient
}

// NewTypeName creates a new TypeName instance
func NewTypeName(param string) *TypeName {
    // Implementation
}

// MethodName does something specific
func (t *TypeName) MethodName() error {
    // Implementation
}

func main() {
    // Entry point
}
```

## File Organization

Kit uses a flat package structure (all files in `package main`):

| File | Responsibility |
|------|----------------|
| `main.go` | Entry point, Slack handlers, core bot logic |
| `gemini.go` | Google Gemini AI client |
| `claude.go` | Anthropic Claude AI client |
| `discord.go` | Discord bot handlers |

### When to Create a New File

- New AI provider → `newprovider.go`
- New platform integration → `platform.go`
- Shared utilities (if needed) → `utils.go`

## Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Files | lowercase | `gemini.go`, `discord.go` |
| Packages | lowercase | `package main` |
| Exported functions | PascalCase | `func NewGeminiClient()` |
| Unexported functions | camelCase | `func handleEvents()` |
| Exported types | PascalCase | `type GeminiClient struct` |
| Struct fields | camelCase | `apiKey string` |
| Constants | PascalCase or camelCase | `MaxRetries` or `maxRetries` |
| Global variables | camelCase with prefix | `globalGeminiClient` |

## Import Organization

Group imports in this order, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"
    "log"
    "os"
    "strings"
    "time"
    
    // Third-party packages
    "github.com/bwmarrin/discordgo"
    "github.com/google/generative-ai-go/genai"
    "github.com/joho/godotenv"
    "github.com/slack-go/slack"
    
    // Local packages (if any)
)
```

## Struct Patterns

### Client Wrapper Pattern

Used for AI clients (Gemini, Claude):

```go
// GeminiClient wraps the Google Generative AI client
type GeminiClient struct {
    client *genai.Client
    model  *genai.GenerativeModel
}

// NewGeminiClient creates a new Gemini client
// Returns nil if configuration is invalid
func NewGeminiClient(apiKey, modelName string) *GeminiClient {
    if apiKey == "" {
        return nil
    }
    
    // Initialize client...
    
    return &GeminiClient{
        client: client,
        model:  model,
    }
}

// GenerateResponse generates a response using Gemini AI
func (g *GeminiClient) GenerateResponse(message string) (string, error) {
    if g == nil || g.client == nil {
        return "", nil  // Return empty to trigger fallback
    }
    
    // Implementation...
}
```

### Bot Configuration Pattern

```go
// Bot holds the configuration and clients for the bot
type Bot struct {
    slackAPI     *slack.Client
    discordBot   *DiscordBot
    geminiClient *GeminiClient
    claudeClient *ClaudeClient
    botUserID    string
    startTime    string
}
```

## Error Handling

### Pattern 1: Return nil for Optional Features

```go
func NewGeminiClient(apiKey, modelName string) *GeminiClient {
    if apiKey == "" {
        return nil  // Not an error - feature just not configured
    }
    // ...
}
```

### Pattern 2: Log and Continue

```go
if err := godotenv.Load(); err != nil {
    log.Println("Warning: .env file not found")
    // Continue anyway - env vars might be set elsewhere
}
```

### Pattern 3: Fatal for Required Config

```go
if slackBotToken == "" && discordToken == "" {
    log.Fatal("❌ At least one platform must be configured")
}
```

### Pattern 4: Return Empty String for Fallback

```go
func (g *GeminiClient) GenerateResponse(message string) (string, error) {
    // Return empty string (not error) to trigger fallback chain
    if g == nil {
        return "", nil
    }
    
    resp, err := g.model.GenerateContent(ctx, genai.Text(message))
    if err != nil {
        log.Printf("❌ Gemini API error: %v", err)
        return "", err  // Error triggers fallback
    }
    
    return extractText(resp), nil
}
```

## Logging Conventions

Use emoji prefixes consistently (matches Python scripts):

```go
log.Printf("🚀 Starting Kit AI Bot...")           // Startup
log.Printf("🔵 Initializing Slack integration...") // Init phase
log.Printf("✅ Connected to Slack!")               // Success
log.Printf("❌ Failed to connect: %v", err)        // Error
log.Printf("⚠️  No AI clients available")          // Warning
log.Printf("📥 Received event: %s", event.Type)   // Inbound
log.Printf("📤 Sending message to %s", channel)   // Outbound
log.Printf("🧠 Gemini response generated")         // AI operations
log.Printf("🔑 Token: %s...", token[:20])         // Credentials (truncated!)
```

### Security: Never Log Full Credentials

```go
// ✅ Good - truncated
log.Printf("🔑 Slack Bot Token: %s...", slackBotToken[:20])

// ❌ Bad - exposes full token
log.Printf("Token: %s", slackBotToken)
```

## Context and Timeouts

Always use context for external API calls:

```go
func (g *GeminiClient) GenerateResponse(message string) (string, error) {
    // 30 second timeout for AI requests
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    resp, err := g.model.GenerateContent(ctx, genai.Text(message))
    // ...
}
```

## Retry Logic

Pattern used in `sendMessage()`:

```go
func sendMessage(api *slack.Client, channel, text string) {
    maxRetries := 3
    for attempt := 1; attempt <= maxRetries; attempt++ {
        _, _, err := api.PostMessage(channel, slack.MsgOptionText(text, false))
        
        if err != nil {
            if attempt < maxRetries {
                log.Printf("⚠️  Failed (attempt %d/%d): %v. Retrying...", 
                    attempt, maxRetries, err)
                time.Sleep(time.Duration(attempt) * time.Second)  // Exponential backoff
                continue
            }
            log.Printf("❌ Failed after %d attempts: %v", maxRetries, err)
            return
        }
        
        log.Println("✅ Message sent successfully!")
        return
    }
}
```

## Event Handler Pattern

Used for Slack Socket Mode events:

```go
func handleEvents(ctx context.Context, client *socketmode.Client, api *slack.Client) {
    for {
        select {
        case <-ctx.Done():
            log.Println("🛑 Shutting down...")
            return
            
        case event := <-client.Events:
            switch event.Type {
            case socketmode.EventTypeConnected:
                log.Println("✅ Connected!")
                
            case socketmode.EventTypeEventsAPI:
                handleEventsAPI(event, client, api)
                
            default:
                log.Printf("❓ Unhandled event: %s", event.Type)
            }
        }
    }
}
```

## Global State

Kit uses package-level variables for shared state:

```go
// Global clients for easy access across handlers
var globalGeminiClient *GeminiClient
var globalClaudeClient *ClaudeClient
var globalBot *Bot
```

**Why**: Simplifies event handlers that need AI clients without passing through many layers.

**Trade-off**: Less testable, but acceptable for a single-binary bot.

## Environment Variables

```go
// Load from .env file (optional)
if err := godotenv.Load(); err != nil {
    log.Println("Warning: .env file not found")
}

// Get with default
geminiModel := os.Getenv("GEMINI_MODEL")
if geminiModel == "" {
    geminiModel = "gemini-1.5-flash"  // default
}

// Required check
if slackBotToken == "" && discordToken == "" {
    log.Fatal("❌ At least one platform must be configured")
}
```

## Testing

Run tests with:

```bash
go test ./...                      # All tests
go test -run TestFunctionName      # Single test
go test -race -cover ./...         # With race detection & coverage
```

Test file naming: `*_test.go` (e.g., `main_test.go`)

## Build Commands

```bash
# Development
go run .

# Production binary
go build -o kit-ai-bot

# Optimized production build (used in Dockerfile)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -a -installsuffix cgo \
    -o kit-ai-bot .
```

## Adding a New AI Provider

1. Create `newprovider.go`
2. Follow the client wrapper pattern:

```go
package main

import (
    "context"
    "log"
    "time"
)

type NewProviderClient struct {
    client *somelib.Client
    model  string
}

func NewNewProviderClient(apiKey, model string) *NewProviderClient {
    if apiKey == "" {
        return nil
    }
    // Initialize...
}

func (n *NewProviderClient) GenerateResponse(message string) (string, error) {
    if n == nil {
        return "", nil
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Call API...
}
```

3. Add to `main.go`:
   - Environment variable loading
   - Global variable
   - Initialization in `main()`
   - Add to fallback chain in `generateResponse()`

## Course Connections

While Kit is primarily Go, these concepts connect to coursework:

| Concept | Go Implementation | Course Link |
|---------|------------------|-------------|
| WebSockets | Slack Socket Mode, Discord | CYBV 326 (Networks) |
| HTTP APIs | AI provider clients | CYBV 326 (Application Layer) |
| Error handling | Retry logic, graceful degradation | CYBV 302 (Reliability) |
| Logging | Structured logs for monitoring | CYBV 381 (Incident Response) |
| Configuration | Environment variables | CYBV 302 (Security) |
