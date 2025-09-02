package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Bot holds the configuration and clients for the bot
type Bot struct {
	slackAPI     *slack.Client
	geminiClient *GeminiClient
	botUserID    string
	startTime    string
}

// Global gemini client for easy access
var globalGeminiClient *GeminiClient
var globalBot *Bot

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
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-1.5-flash" // default model
	}

	// Create Bot instance with configuration
	bot := &Bot{
		startTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	globalBot = bot

	// Initialize Gemini client if API key is available
	if geminiAPIKey != "" {
		bot.geminiClient = NewGeminiClient(geminiAPIKey, geminiModel)
		globalGeminiClient = bot.geminiClient
		if bot.geminiClient != nil {
			log.Printf("🧠 Gemini AI initialized with model: %s", geminiModel)
		}
	} else {
		log.Println("⚠️  No Gemini API key found - using basic responses")
	}

	// Create Slack API client with both tokens
	api := slack.New(botToken, slack.OptionDebug(false), slack.OptionAppLevelToken(appToken))
	bot.slackAPI = api

	// Test Slack connection and get bot info
	authTest, err := api.AuthTest()
	if err != nil {
		log.Fatalf("❌ Failed to authenticate with Slack: %v", err)
	}
	bot.botUserID = authTest.UserID
	log.Printf("✅ Authenticated as: %s (ID: %s)", authTest.User, authTest.UserID)

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
				log.Println("⚡ Slash command received!")
				handleSlashCommand(event, client, api)

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

// handleSlashCommand processes slash command events
func handleSlashCommand(event socketmode.Event, client *socketmode.Client, api *slack.Client) {
	// Acknowledge the event first
	if event.Request != nil {
		defer client.Ack(*event.Request)
	}

	// Parse the slash command
	cmd, ok := event.Data.(slack.SlashCommand)
	if !ok {
		log.Printf("❌ Failed to parse slash command: %T", event.Data)
		return
	}

	log.Printf("⚡ Slash command: %s %s from %s in %s", cmd.Command, cmd.Text, cmd.UserName, cmd.ChannelName)

	// Generate response based on command
	response := handleSlashCommandLogic(cmd)

	// Send response back to Slack
	if response != "" {
		sendSlashCommandResponse(api, cmd.ResponseURL, cmd.ChannelID, response)
	}
}

// handleSlashCommandLogic processes the actual slash command logic
func handleSlashCommandLogic(cmd slack.SlashCommand) string {
	commandText := strings.TrimSpace(cmd.Text)

	switch cmd.Command {
	case "/kit":
		return handleKitCommand(commandText, cmd.UserID)
	default:
		return fmt.Sprintf("❓ Unknown command: %s", cmd.Command)
	}
}

// handleKitCommand processes /kit subcommands
func handleKitCommand(args, userID string) string {
	if args == "" {
		return "👋 **Kit Slash Commands**\n\n" +
			"Available commands:\n" +
			"• `/kit status` - Check bot health\n" +
			"• `/kit help` - Show help information\n" +
			"• `/kit version` - Show version info\n" +
			"• `/kit ask [question]` - Ask Kit a question\n\n" +
			"Example: `/kit ask What is Go programming?`"
	}

	parts := strings.Fields(args)
	subcommand := strings.ToLower(parts[0])

	switch subcommand {
	case "status", "health":
		return handleSpecialCommands("status")

	case "help":
		return handleSpecialCommands("help")

	case "version":
		return handleSpecialCommands("version")

	case "ask":
		if len(parts) < 2 {
			return "❓ **Usage:** `/kit ask [your question]`\n\nExample: `/kit ask What is artificial intelligence?`"
		}
		question := strings.Join(parts[1:], " ")
		return generateResponse(question, userID)

	default:
		return fmt.Sprintf("❓ **Unknown subcommand:** `%s`\n\n"+
			"Available commands:\n"+
			"• `/kit status` - Check bot health\n"+
			"• `/kit help` - Show help\n"+
			"• `/kit version` - Show version\n"+
			"• `/kit ask [question]` - Ask a question", subcommand)
	}
}

// sendSlashCommandResponse sends a response to a slash command
func sendSlashCommandResponse(api *slack.Client, responseURL, channelID, text string) {
	log.Printf("📤 Sending slash command response to %s", channelID)

	// For slash commands, we can send an immediate response
	_, _, err := api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionAsUser(true),
	)

	if err != nil {
		log.Printf("❌ Failed to send slash command response: %v", err)
	} else {
		log.Println("✅ Slash command response sent successfully!")
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

	// Remove bot mention from message text
	cleanMessage := removeBotMention(event.Text)

	response := generateResponse(cleanMessage, event.User)
	sendMessage(api, event.Channel, response)
}

// removeBotMention removes bot mention tags from message text
func removeBotMention(text string) string {
	// Remove mentions like <@U123456789> and clean up
	cleanText := text
	if strings.Contains(cleanText, "<@") {
		parts := strings.Split(cleanText, ">")
		if len(parts) > 1 {
			cleanText = strings.Join(parts[1:], ">")
		}
	}

	return strings.TrimSpace(cleanText)
}

// generateResponse creates a response to user messages with AI integration
func generateResponse(message, userID string) string {
	// Clean the message text
	cleanMessage := strings.TrimSpace(message)

	// Remove mention tags like <@U123456789>
	cleanMessage = strings.ReplaceAll(cleanMessage, fmt.Sprintf("<@%s>", userID), "")
	cleanMessage = strings.TrimSpace(cleanMessage)

	log.Printf("💭 Generating response for: '%s'", cleanMessage)

	// Check for special commands first
	if response := handleSpecialCommands(cleanMessage); response != "" {
		return response
	}

	// Try Gemini AI first
	if globalGeminiClient != nil {
		if response, err := globalGeminiClient.GenerateResponse(cleanMessage); err == nil && response != "" {
			log.Printf("🧠 Gemini response generated successfully")
			return response
		} else if err != nil {
			log.Printf("⚠️  Gemini error, falling back to basic response: %v", err)
		}
	}

	// Fallback to basic responses
	return generateBasicResponse(cleanMessage)
}

// handleSpecialCommands processes special bot commands
func handleSpecialCommands(message string) string {
	cleanMessage := strings.ToLower(strings.TrimSpace(message))

	switch {
	case cleanMessage == "status" || cleanMessage == "health":
		aiStatus := "❌ Offline"
		if globalGeminiClient != nil {
			aiStatus = "✅ Online (Gemini)"
		}

		return fmt.Sprintf("🤖 **Kit Status Report**\n"+
			"• Bot Status: ✅ Online and Connected\n"+
			"• AI Engine: %s\n"+
			"• Started: %s\n"+
			"• Ready to help! 🚀", aiStatus, globalBot.startTime)

	case cleanMessage == "help" || cleanMessage == "commands":
		return "🤖 **Kit Commands**\n\n" +
			"**Special Commands:**\n" +
			"• `status` - Check bot health\n" +
			"• `help` - Show this help message\n" +
			"• `version` - Show version info\n\n" +
			"**Slash Commands:**\n" +
			"• `/kit status` - Check bot health\n" +
			"• `/kit help` - Show help\n" +
			"• `/kit ask [question]` - Ask Kit anything\n\n" +
			"**How to use Kit:**\n" +
			"• Send direct messages for private conversations\n" +
			"• Mention @Kit in channels to get responses\n" +
			"• Use slash commands for quick interactions\n" +
			"• Ask questions, request help, or just chat!\n\n" +
			"I'm powered by Google Gemini AI! 🧠✨"

	case strings.Contains(cleanMessage, "version"):
		return "🤖 **Kit v1.0**\n" +
			"• Built with Go\n" +
			"• Powered by Google Gemini AI\n" +
			"• Socket Mode for real-time responses\n" +
			"• Open source and ready to help! 🚀"
	}

	return "" // No special command matched
}

// generateBasicResponse provides fallback responses when AI is unavailable
func generateBasicResponse(cleanMessage string) string {
	cleanMessage = strings.ToLower(cleanMessage)

	switch {
	case strings.Contains(cleanMessage, "hello") || strings.Contains(cleanMessage, "hi") || strings.Contains(cleanMessage, "hey"):
		return "👋 **Hello there!** I'm Kit, your AI assistant.\n\n" +
			"I'm powered by Google Gemini AI and ready to help with questions, conversations, or tasks!\n\n" +
			"Try asking me something or type `help` for available commands. 🤖✨"

	case strings.Contains(cleanMessage, "how are you"):
		return "😊 **I'm doing great, thanks for asking!**\n\n" +
			"I'm running smoothly with my Gemini AI brain engaged and ready to tackle whatever you throw at me!\n\n" +
			"How are you doing today? Anything I can help you with? �"

	case strings.Contains(cleanMessage, "what can you do") || strings.Contains(cleanMessage, "capabilities"):
		return "🚀 **Here's what I can do:**\n\n" +
			"✅ **Answer complex questions** - I can help with explanations, research, and analysis\n" +
			"✅ **Have intelligent conversations** - Chat about any topic that interests you\n" +
			"✅ **Provide coding help** - Assistance with programming and technical questions\n" +
			"✅ **Creative tasks** - Writing, brainstorming, and creative problem-solving\n" +
			"✅ **General assistance** - Help with work, learning, and daily tasks\n\n" +
			"Just ask me anything! I'm powered by Google Gemini AI. 🧠�"

	case strings.Contains(cleanMessage, "thank"):
		return "😊 **You're very welcome!** Happy to help anytime!\n\n" +
			"Feel free to ask me anything else - I'm here and ready to assist! 🤖�"

	case strings.Contains(cleanMessage, "what day") || strings.Contains(cleanMessage, "what time") || strings.Contains(cleanMessage, "date"):
		return "📅 **Time & Date Info:**\n\n" +
			"I don't have access to real-time information right now, but I can help you with:\n" +
			"• General time zone questions\n" +
			"• Date calculations\n" +
			"• Calendar-related queries\n\n" +
			"For current time, check your system clock! ⏰"

	case strings.Contains(cleanMessage, "video game") || strings.Contains(cleanMessage, "game") || strings.Contains(cleanMessage, "gaming"):
		return "🎮 **Gaming Discussion!**\n\n" +
			"I'd love to chat about video games! I can help with:\n" +
			"• Game recommendations\n" +
			"• Gaming strategies and tips\n" +
			"• Industry news and trends\n" +
			"• Gaming technology\n\n" +
			"What type of games are you into? 🕹️"

	case strings.Contains(cleanMessage, "weather"):
		return "🌤️ **Weather Info:**\n\n" +
			"I don't have access to live weather data, but I can help with:\n" +
			"• Weather-related questions\n" +
			"• Climate information\n" +
			"• Weather patterns and phenomena\n\n" +
			"Check your local weather app for current conditions! ☀️"

	default:
		return fmt.Sprintf("🤖 **Message received:** \"%s\"\n\n"+
			"I'm Kit, your AI assistant! I can help with questions, conversations, and various tasks.\n\n"+
			"💡 **Try asking me about:**\n"+
			"• Any topic you're curious about\n"+
			"• Help with work or projects\n"+
			"• Creative brainstorming\n"+
			"• Technical questions\n\n"+
			"Or type `help` to see available commands! ✨", cleanMessage)
	}
}

// sendMessage sends a message to a Slack channel with enhanced error handling
func sendMessage(api *slack.Client, channel, text string) {
	log.Printf("📤 Sending message to %s: %.100s%s", channel, text, func() string {
		if len(text) > 100 {
			return "..."
		}
		return ""
	}())

	// Add retry logic for failed sends
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, _, err := api.PostMessage(
			channel,
			slack.MsgOptionText(text, false),
			slack.MsgOptionAsUser(true),
		)

		if err != nil {
			if attempt < maxRetries {
				log.Printf("⚠️  Failed to send message (attempt %d/%d): %v. Retrying...", attempt, maxRetries, err)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			} else {
				log.Printf("❌ Failed to send message after %d attempts: %v", maxRetries, err)
				return
			}
		} else {
			if attempt > 1 {
				log.Printf("✅ Message sent successfully on attempt %d!", attempt)
			} else {
				log.Println("✅ Message sent successfully!")
			}
			return
		}
	}
}
