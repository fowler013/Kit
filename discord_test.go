package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
		{"!kit hello", false, false, "Hello there"},
		{"!KiT !help", false, false, "Kit Discord Commands"},
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
	response := bot.generateDiscordResponse("who registered for camp?", "123", "456", false, nil)
	if called || !strings.Contains(response, "not configured") {
		t.Fatal("disabled camp query was not kept out of the AI provider")
	}
}

func TestCampDataStaysInPrivateChannels(t *testing.T) {
	var exports atomic.Int32
	camp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/export-json" {
			http.NotFound(w, r)
			return
		}
		exports.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"registrations":[{"child_first_name":"Synthetic","child_last_name":"Camper"}]}`)
	}))
	defer camp.Close()
	oldCamp := globalCampClient
	t.Cleanup(func() { globalCampClient = oldCamp })

	const (
		data   = "Registration Stats"
		notice = "For privacy"
		denied = "restricted"
	)
	for _, test := range []struct {
		name, allowed, user, guild, channel, content, want string
		lookups                                            int
	}{
		{"staff channel", "555, 777", "123", "camp", "555", "!camp stats", data, 0},
		{"second staff channel", "555, 777", "123", "camp", "777", "!camp stats", data, 0},
		{"public channel", "555, 777", "123", "camp", "999", "!camp stats", notice, 0},
		{"natural question in public channel", "555, 777", "123", "camp", "999", "<@99> who registered for camp?", notice, 0},
		{"direct message", "555, 777", "123", "", "dm", "!camp stats", data, 0},
		{"thread in staff channel", "555, 777", "123", "camp", "888", "!camp stats", data, 0},
		{"thread in public channel", "555, 777", "123", "camp", "889", "!camp stats", notice, 0},
		{"thread missing from cache", "555, 777", "123", "camp", "890", "!camp stats", data, 1},
		{"unknown channel fails closed", "555, 777", "123", "camp", "891", "!camp stats", notice, 1},
		{"no staff channels configured", "", "123", "camp", "555", "!camp stats", notice, 0},
		{"no staff channels still allows DMs", "", "123", "", "dm", "!camp stats", data, 0},
		{"unauthorized in staff channel", "555, 777", "456", "camp", "555", "!camp stats", denied, 0},
		{"unauthorized in public channel", "555, 777", "456", "camp", "999", "!camp stats", denied, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var aiCalled atomic.Bool
			service := NewAIService(nil, nil, providerFunc{name: "synthetic", fn: func(context.Context, string, *Session) (string, error) {
				aiCalled.Store(true)
				return "ai answer", nil
			}})
			bot, err := NewDiscordBot("synthetic", nil, nil, "test", service)
			if err != nil {
				t.Fatal(err)
			}
			bot.botID = "99"
			if err := bot.session.State.GuildAdd(&discordgo.Guild{
				ID: "camp",
				Channels: []*discordgo.Channel{
					{ID: "555", GuildID: "camp", Type: discordgo.ChannelTypeGuildText},
					{ID: "777", GuildID: "camp", Type: discordgo.ChannelTypeGuildText},
					{ID: "999", GuildID: "camp", Type: discordgo.ChannelTypeGuildText},
				},
				Threads: []*discordgo.Channel{
					{ID: "888", GuildID: "camp", ParentID: "555", Type: discordgo.ChannelTypeGuildPublicThread},
					{ID: "889", GuildID: "camp", ParentID: "999", Type: discordgo.ChannelTypeGuildPublicThread},
				},
			}); err != nil {
				t.Fatal(err)
			}

			globalCampClient = NewCampClient(camp.URL, "staff", "test", "123", "", 30)
			globalCampClient.SetAllowedChannels(test.allowed)
			exports.Store(0)

			var messages []discordgo.MessageSend
			lookups := 0
			bot.session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				reply := func(status int, body string) (*http.Response, error) {
					return &http.Response{
						StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
						Body: io.NopCloser(strings.NewReader(body)),
					}, nil
				}
				switch {
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
					var message discordgo.MessageSend
					if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
						t.Error(err)
					}
					messages = append(messages, message)
					return reply(200, `{"id":"response","content":"ok"}`)
				case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v9/channels/"):
					lookups++
					if strings.TrimPrefix(r.URL.Path, "/api/v9/channels/") == "890" {
						return reply(200, `{"id":"890","type":11,"guild_id":"camp","parent_id":"555"}`)
					}
					return reply(404, `{"message":"Unknown Channel","code":10003}`)
				}
				t.Errorf("unexpected Discord request: %s %s", r.Method, r.URL.Path)
				return reply(500, `{}`)
			})

			bot.onMessageCreate(bot.session, &discordgo.MessageCreate{Message: &discordgo.Message{
				Content: test.content, GuildID: test.guild, ChannelID: test.channel,
				Author: &discordgo.User{ID: test.user},
			}})

			if len(messages) != 1 || !strings.Contains(messages[0].Content, test.want) {
				t.Fatalf("response=%+v, want text containing %q", messages, test.want)
			}
			wantExports := int32(0)
			if test.want == data {
				wantExports = 1
			}
			if got := exports.Load(); got != wantExports {
				t.Errorf("registration exports=%d, want %d", got, wantExports)
			}
			if lookups != test.lookups {
				t.Errorf("channel REST lookups=%d, want %d", lookups, test.lookups)
			}
			if aiCalled.Load() {
				t.Error("camp question reached the AI provider")
			}
		})
	}
}
