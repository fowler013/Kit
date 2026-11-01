package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDiscordGuildCommandRouting(t *testing.T) {
	oldCamp := globalCampClient
	globalCampClient = nil
	t.Cleanup(func() { globalCampClient = oldCamp })
	for _, test := range []struct {
		content string
		dm      bool
		bot     bool
		want    string
	}{
		{"!help", false, false, "Kit Discord Commands"},
		{"!status", false, false, "Basic responses only"},
		{"!myid", false, false, "123"},
		{" !HeLp ", false, false, "Kit Discord Commands"},
		{"!camp roster", false, false, "not configured"},
		{"!links", false, false, "No links configured"},
		{"<@99> !help", false, false, "Kit Discord Commands"},
		{"<@!99> !help", false, false, "Kit Discord Commands"},
		{"hello", true, false, "Hello there"},
		{"hello", false, false, ""},
		{"!another-bot-command", false, false, ""},
		{"!help", false, true, ""},
	} {
		t.Run(test.content+fmt.Sprint(test.dm, test.bot), func(t *testing.T) {
			bot, err := NewDiscordBot("synthetic", nil, nil, "test", nil)
			if err != nil {
				t.Fatal(err)
			}
			bot.botID = "99"
			var messages []discordgo.MessageSend
			bot.session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/messages") {
					t.Errorf("unexpected Discord request: %s %s", r.Method, r.URL.Path)
				}
				var message discordgo.MessageSend
				if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
					t.Error(err)
				}
				messages = append(messages, message)
				return &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"id":"response","content":"ok"}`)),
				}, nil
			})
			guildID := "camp"
			if test.dm {
				guildID = ""
			}
			bot.onMessageCreate(bot.session, &discordgo.MessageCreate{Message: &discordgo.Message{
				Content: test.content, GuildID: guildID, ChannelID: "channel",
				Author: &discordgo.User{ID: "123", Bot: test.bot},
			}})
			if test.want == "" {
				if len(messages) != 0 {
					t.Fatal("unrelated or bot message received a response")
				}
				return
			}
			if len(messages) != 1 || !strings.Contains(messages[0].Content, test.want) {
				t.Fatalf("unexpected response: %+v", messages)
			}
			if messages[0].AllowedMentions == nil || len(messages[0].AllowedMentions.Parse) != 0 {
				t.Fatal("response could notify unsolicited mentions")
			}
		})
	}
}

func TestDiscordIntentsAndReadiness(t *testing.T) {
	bot, err := NewDiscordBot("synthetic", nil, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	if bot.session.Identify.Intents != want {
		t.Fatalf("intents=%d, want %d", bot.session.Identify.Intents, want)
	}
	if bot.Ready() {
		t.Fatal("new bot must not be ready")
	}
	bot.botID = "99"
	bot.session.DataReady = true
	if !bot.Ready() {
		t.Fatal("connected gateway must be ready")
	}
	bot.session.DataReady = false
	if bot.Ready() {
		t.Fatal("disconnected gateway must not be ready")
	}
	bot.onMessageCreate(bot.session, nil)
	bot.onMessageCreate(bot.session, &discordgo.MessageCreate{})
}

func TestDisabledCampDoesNotReachAI(t *testing.T) {
	oldCamp := globalCampClient
	globalCampClient = nil
	defer func() { globalCampClient = oldCamp }()
	called := false
	service := NewAIService(nil, nil, providerFunc{name: "synthetic", fn: func(context.Context, string, *Session) (string, error) {
		called = true
		return "should not be called", nil
	}})
	bot := &DiscordBot{aiService: service}
	response := bot.generateDiscordResponse("who registered for camp?", "123", "456", false)
	if called || !strings.Contains(response, "not configured") {
		t.Fatal("disabled camp query was not kept out of the AI provider")
	}
}
