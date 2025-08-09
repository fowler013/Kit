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

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	token := os.Getenv("SLACK_BOT_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	if token == "" || appToken == "" {
		log.Fatal("SLACK_BOT_TOKEN and SLACK_APP_TOKEN must be set")
	}

	// Create a new client to slack by giving token
	api := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))

	// Create socket mode client
	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Create a context that can be used to cancel goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start listening to events
	go func(ctx context.Context, client *socketmode.Client, api *slack.Client) {
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down socketmode listener")
				return
			case event := <-client.Events:
				switch event.Type {
				case socketmode.EventTypeEventsAPI:
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
						continue
					}

					// Acknowledge the event
					client.Ack(*event.Request)

					// Handle the inner event
					err := handleEventMessage(eventsAPIEvent, api)
					if err != nil {
						log.Printf("Failed to handle message: %v", err)
					}
				}
			}
		}
	}(ctx, socketClient, api)

	log.Println("Starting Slack AI Bot...")
	err = socketClient.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// handleEventMessage handles incoming events from Slack
func handleEventMessage(event slackevents.EventsAPIEvent, api *slack.Client) error {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			// Don't respond to bot messages
			if ev.SubType == "bot_message" {
				return nil
			}

			// Only respond to direct messages or when the bot is mentioned
			if ev.Channel[0] == 'D' || strings.Contains(ev.Text, "<@") {
				response := generateAIResponse(ev.Text)
				_, _, err := api.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
				if err != nil {
					return fmt.Errorf("failed to post message: %w", err)
				}
			}
		}
	default:
		return fmt.Errorf("unsupported event type: %s", event.Type)
	}
	return nil
}

// generateAIResponse generates an AI response to the user's message
// This is a placeholder - you can integrate with OpenAI, Claude, or other AI services here
func generateAIResponse(message string) string {
	// Simple echo bot for now
	// TODO: Integrate with your preferred AI service (OpenAI, Claude, etc.)
	
	if strings.Contains(strings.ToLower(message), "hello") {
		return "Hello! I'm your AI assistant. How can I help you today?"
	}
	
	if strings.Contains(strings.ToLower(message), "help") {
		return "I'm an AI bot that can help answer questions and have conversations. Just send me a message!"
	}
	
	return fmt.Sprintf("I received your message: %s\n\nI'm a simple AI bot. To make me smarter, integrate me with an AI service like OpenAI or Claude!", message)
}
