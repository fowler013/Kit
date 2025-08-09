package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Bot holds the configuration and clients for the bot
type Bot struct {
	slackAPI     *slack.Client
	claudeClient *ClaudeClient
}

// Global claude client for easy access
var globalClaudeClient *ClaudeClient

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get required tokens
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	if botToken == "" || appToken == "" {
		log.Fatal("❌ SLACK_BOT_TOKEN and SLACK_APP_TOKEN must be set in .env file")
	}

	log.Printf("🚀 Starting Kit AI Bot...")
	log.Printf("🔑 Bot Token: %s...", botToken[:20])
	log.Printf("🔑 App Token: %s...", appToken[:20])

	// Get AI configuration
	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	anthropicModel := os.Getenv("ANTHROPIC_MODEL")
	if anthropicModel == "" {
		anthropicModel = "claude-3-sonnet-20240229" // default model
	}

	// Create Bot instance with configuration
	bot := &Bot{}

	// Initialize Claude client if API key is available
	if anthropicAPIKey != "" {
		bot.claudeClient = NewClaudeClient(anthropicAPIKey, anthropicModel)
		globalClaudeClient = bot.claudeClient
		if bot.claudeClient != nil {
			log.Printf("🧠 Claude AI initialized with model: %s", anthropicModel)
		}
	} else {
		log.Println("⚠️  No Anthropic API key found - using basic responses")
	}

	// Create Slack API client with both tokens
	api := slack.New(botToken, slack.OptionDebug(false), slack.OptionAppLevelToken(appToken))
	bot.slackAPI = api

	// Create Socket Mode client
	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(false),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("📡 Starting event listener...")

	// Start event handler goroutine
	go handleEvents(ctx, socketClient, api)

	// Start the Socket Mode connection
	log.Println("🔌 Connecting to Slack...")
	if err := socketClient.Run(); err != nil {
		log.Fatalf("❌ Failed to start Socket Mode: %v", err)
	}
}

// handleEvents processes all incoming Slack events
func handleEvents(ctx context.Context, client *socketmode.Client, api *slack.Client) {
	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Shutting down event listener...")
			return

		case event := <-client.Events:
			log.Printf("📥 Received event: %s", event.Type)

			switch event.Type {
			case socketmode.EventTypeConnecting:
				log.Println("🔄 Connecting to Slack...")

			case socketmode.EventTypeConnectionError:
				log.Println("❌ Connection error!")
				if event.Request != nil {
					client.Ack(*event.Request)
				}

			case socketmode.EventTypeConnected:
				log.Println("✅ Connected to Slack!")

			case socketmode.EventTypeHello:
				log.Println("👋 Received hello from Slack")

			case socketmode.EventTypeEventsAPI:
				log.Println("🎉 EventsAPI event received!")
				handleEventsAPI(event, client, api)

			case socketmode.EventTypeSlashCommand:
				log.Println("⚡ Slash command received (not implemented)")
				if event.Request != nil {
					client.Ack(*event.Request)
				}

			case socketmode.EventTypeInteractive:
				log.Println("🎮 Interactive event received (not implemented)")
				if event.Request != nil {
					client.Ack(*event.Request)
				}

			default:
				log.Printf("❓ Unhandled event type: %s", event.Type)
				if event.Request != nil {
					client.Ack(*event.Request)
				}
			}
		}
	}
}

// handleEventsAPI processes EventsAPI events (messages, mentions, etc.)
func handleEventsAPI(event socketmode.Event, client *socketmode.Client, api *slack.Client) {
	// Acknowledge the event first
	if event.Request != nil {
		client.Ack(*event.Request)
	}

	// Parse the EventsAPI event
	eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
	if !ok {
		log.Printf("❌ Failed to parse EventsAPI event: %T", event.Data)
		return
	}

	log.Printf("📋 EventsAPI - Type: %s, Team: %s", eventsAPIEvent.Type, eventsAPIEvent.TeamID)

	// Handle the inner event
	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		log.Println("📞 Processing callback event...")
		handleCallbackEvent(eventsAPIEvent.InnerEvent, api)

	case slackevents.URLVerification:
		log.Println("🔗 URL verification event (not needed in Socket Mode)")

	default:
		log.Printf("❓ Unhandled EventsAPI type: %s", eventsAPIEvent.Type)
	}
}

// handleCallbackEvent processes callback events (messages, mentions, etc.)
func handleCallbackEvent(innerEvent slackevents.EventsAPIInnerEvent, api *slack.Client) {
	log.Printf("📱 Inner event type: %T", innerEvent.Data)

	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		log.Printf("💬 Message received: '%s' from %s in %s", ev.Text, ev.User, ev.Channel)
		handleMessageEvent(ev, api)

	case *slackevents.AppMentionEvent:
		log.Printf("📢 Kit mentioned: '%s' from %s in %s", ev.Text, ev.User, ev.Channel)
		handleMentionEvent(ev, api)

	default:
		log.Printf("❓ Unhandled inner event type: %T", innerEvent.Data)
	}
}

// handleMessageEvent processes regular message events
func handleMessageEvent(event *slackevents.MessageEvent, api *slack.Client) {
	// Skip bot messages to avoid loops
	if event.SubType == "bot_message" || event.BotID != "" {
		log.Println("🤖 Skipping bot message")
		return
	}

	// Only respond to direct messages (DM channels start with 'D')
	if strings.HasPrefix(event.Channel, "D") {
		log.Println("📨 Direct message - generating response...")
		response := generateResponse(event.Text, event.User)
		sendMessage(api, event.Channel, response)
	} else {
		log.Printf("👀 Public channel message ignored (channel: %s)", event.Channel)
	}
}

// handleMentionEvent processes app mention events
func handleMentionEvent(event *slackevents.AppMentionEvent, api *slack.Client) {
	log.Printf("🎯 Kit mentioned in channel %s", event.Channel)
	response := generateResponse(event.Text, event.User)
	sendMessage(api, event.Channel, response)
}

// generateResponse creates a response to user messages with AI integration
func generateResponse(message, userID string) string {
	// Clean the message text
	cleanMessage := strings.TrimSpace(message)
	
	// Remove mention tags like <@U123456789>
	cleanMessage = strings.ReplaceAll(cleanMessage, fmt.Sprintf("<@%s>", userID), "")
	cleanMessage = strings.TrimSpace(cleanMessage)

	log.Printf("💭 Generating response for: '%s'", cleanMessage)

	// Try Claude AI first
	if globalClaudeClient != nil {
		if response, err := globalClaudeClient.GenerateResponse(cleanMessage); err == nil && response != "" {
			log.Printf("🧠 Claude response generated successfully")
			return response
		} else if err != nil {
			log.Printf("⚠️  Claude error, falling back to basic response: %v", err)
		}
	}

	// Fallback to basic responses
	return generateBasicResponse(cleanMessage)
}

// generateBasicResponse provides fallback responses when AI is unavailable
func generateBasicResponse(cleanMessage string) string {
	cleanMessage = strings.ToLower(cleanMessage)

	switch {
	case strings.Contains(cleanMessage, "hello") || strings.Contains(cleanMessage, "hi"):
		return "Hello! I'm Kit, your AI assistant. How can I help you today? 👋"

	case strings.Contains(cleanMessage, "help"):
		return "I'm Kit, your AI bot! I can:\n• Answer questions\n• Have conversations\n• Help with various tasks\n\nJust ask me anything! 🤖"

	case strings.Contains(cleanMessage, "how are you"):
		return "I'm doing great! Thanks for asking. How are you doing? 😊"

	case strings.Contains(cleanMessage, "what can you do"):
		return "I'm Kit, and I can help you with:\n• Answering questions\n• Having conversations\n• Providing assistance\n• And much more!\n\nWhat would you like to know? 🚀"

	case strings.Contains(cleanMessage, "thank"):
		return "You're very welcome! Happy to help! 😊"

	default:
		return fmt.Sprintf("Hi there! I received your message: \"%s\"\n\nI'm Kit, your AI assistant. How can I help you today? 🤖", cleanMessage)
	}
}

// sendMessage sends a message to a Slack channel
func sendMessage(api *slack.Client, channel, text string) {
	log.Printf("📤 Sending message to %s: %s", channel, text)

	_, _, err := api.PostMessage(
		channel,
		slack.MsgOptionText(text, false),
		slack.MsgOptionAsUser(true),
	)

	if err != nil {
		log.Printf("❌ Failed to send message: %v", err)
	} else {
		log.Println("✅ Message sent successfully!")
	}
}
