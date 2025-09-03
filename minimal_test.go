package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main1() {
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

	fmt.Printf("🔧 Bot Token: %s...\n", token[:20])
	fmt.Printf("🔧 App Token: %s...\n", appToken[:20])

	// Create Slack API client
	api := slack.New(token, slack.OptionDebug(false))

	// Create Socket Mode client
	socketClient := socketmode.New(api, socketmode.OptionDebug(false))

	fmt.Println("🚀 Starting minimal test bot...")
	fmt.Println("📡 Listening for ALL events...")

	// Handle events
	go func() {
		for evt := range socketClient.Events {
			fmt.Printf("\n🎯 RAW EVENT RECEIVED: %s\n", evt.Type)
			fmt.Printf("📋 Event Data Type: %T\n", evt.Data)

			switch evt.Type {
			case socketmode.EventTypeConnecting:
				fmt.Println("🔄 Connecting...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("❌ Connection Error")
			case socketmode.EventTypeConnected:
				fmt.Println("✅ Connected!")
			case socketmode.EventTypeEventsAPI:
				fmt.Println("🎉 EVENTS API EVENT RECEIVED!")

				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("❌ Type assertion failed: %T\n", evt.Data)
					socketClient.Ack(*evt.Request)
					continue
				}

				fmt.Printf("📨 EventsAPI Type: %s\n", eventsAPIEvent.Type)
				fmt.Printf("📨 Inner Event Type: %T\n", eventsAPIEvent.InnerEvent.Data)

				// Handle specific events
				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.MessageEvent:
						fmt.Printf("💬 MESSAGE: %s (from %s in %s)\n", ev.Text, ev.User, ev.Channel)

						// Simple response
						api.PostMessage(ev.Channel, slack.MsgOptionText("I got your message!", false))

					case *slackevents.AppMentionEvent:
						fmt.Printf("📢 MENTION: %s (from %s in %s)\n", ev.Text, ev.User, ev.Channel)

						// Simple response
						api.PostMessage(ev.Channel, slack.MsgOptionText("Hello! I heard you mention me!", false))
					}
				}

				socketClient.Ack(*evt.Request)

			case socketmode.EventTypeHello:
				fmt.Println("👋 Hello event (connection established)")
			default:
				fmt.Printf("❓ Unknown event type: %s\n", evt.Type)
				if evt.Request != nil {
					socketClient.Ack(*evt.Request)
				}
			}
		}
	}()

	// Start the connection
	err = socketClient.Run()
	if err != nil {
		log.Fatal(err)
	}
}
