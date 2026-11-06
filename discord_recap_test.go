package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const recapTestChannel = "223456789012345678"

func recapOption(channel string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name: "recap", Type: discordgo.ApplicationCommandOptionSubCommand,
		Options: []*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: channel,
		}},
	}
}

func recapMessage(id string, at time.Time, user, content string) *discordgo.Message {
	return &discordgo.Message{
		ID: id, Timestamp: at, Author: &discordgo.User{ID: user, Username: "User" + user}, Content: content,
	}
}

func TestAnonymizeRecap(t *testing.T) {
	first := &discordgo.User{ID: slashTestParent, Username: "alice_handle", GlobalName: "Alice"}
	second := &discordgo.User{ID: slashTestStaff, Username: "bob_handle", GlobalName: "Bob"}
	messages := []*discordgo.Message{
		{Author: first, Member: &discordgo.Member{Nick: "Alicia"}, Mentions: []*discordgo.User{second},
			Content: "Alice Alicia alice_handle Bob bob_handle <@" + second.ID + "> <@!" + first.ID + "> <@&123456789012345670> <#123456789012345671> @everyone @here " +
				"https://example.com/alice alice@example.com +1 (555) 123-4567 555.987.6543",
			Attachments: []*discordgo.MessageAttachment{{URL: "https://private.example/file", Filename: "Alice.png"}}},
		{Author: second, Content: "Minecraft is fun"},
		{Author: first, Content: "Pokemon too"},
	}
	got := anonymizeRecap(messages)
	for _, private := range []string{"Alice", "Alicia", "alice_handle", "Bob", "bob_handle", slashTestParent, slashTestStaff,
		"<@", "<#", "@everyone", "@here", "https", "example.com", "555", "123-4567", "Alice.png"} {
		if strings.Contains(got, private) {
			t.Errorf("anonymizer leaked %q: %s", private, got)
		}
	}
	for _, marker := range []string{"Camper 1:", "Camper 2: Minecraft is fun", "Camper 1: Pokemon too", "[mention]", "[link]", "[attachment]", "[removed]"} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing %q: %s", marker, got)
		}
	}
	unicodeUser := &discordgo.User{ID: "unicode", Username: "\u00c9milie"}
	if got := anonymizeRecap([]*discordgo.Message{{Author: unicodeUser, Content: "\u00c9milie likes Minecraft"}}); strings.Contains(got, "\u00c9milie") {
		t.Fatal("Unicode name leaked")
	}
}

func TestRecapInputBoundsAndRedactionBeforeTruncation(t *testing.T) {
	messages := []*discordgo.Message{}
	for n := 0; n < 100; n++ {
		messages = append(messages, &discordgo.Message{Author: &discordgo.User{ID: "one", Username: "PrivateName"},
			Content:     strings.Repeat("x", 270) + " https://example.com/" + strings.Repeat("sensitive", 100),
			Attachments: []*discordgo.MessageAttachment{{Filename: "secret.png"}}})
	}
	got := anonymizeRecap(messages)
	if utf8.RuneCountInString(got) > maxRecapInput || strings.Contains(got, "sensitive") || strings.Contains(got, "https") {
		t.Fatal("input cap/redaction failed")
	}
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if utf8.RuneCountInString(line) > 320 || !strings.Contains(line, "[attachment]") {
			t.Fatal("per-message bound/attachment marker failed")
		}
	}
}

func TestFetchRecapPagination(t *testing.T) {
	now := time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC)
	t.Run("100 inspected with oldest first", func(t *testing.T) {
		calls := 0
		got, err := fetchRecapMessages(context.Background(), now, func(ctx context.Context, before string, limit int) ([]*discordgo.Message, error) {
			if limit != 50 || (calls == 0 && before != "") || (calls == 1 && before != "50") {
				t.Fatalf("bad pagination before=%s limit=%d", before, limit)
			}
			page := []*discordgo.Message{}
			for n := 1; n <= 50; n++ {
				index := calls*50 + n
				page = append(page, recapMessage(fmt.Sprint(index), now.Add(-time.Duration(index)*time.Minute), "person", fmt.Sprint(index)))
			}
			calls++
			return page, nil
		})
		if err != nil || len(got) != 100 || calls != 2 || got[0].ID != "100" || got[99].ID != "1" {
			t.Fatalf("pagination result len=%d calls=%d err=%v", len(got), calls, err)
		}
	})
	t.Run("cutoff boundary and human filters", func(t *testing.T) {
		bot := recapMessage("bot", now, "bot", "skip")
		bot.Author.Bot = true
		webhook := recapMessage("webhook", now, "hook", "skip")
		webhook.WebhookID = "webhook"
		system := recapMessage("system", now, "system", "skip")
		system.Type = discordgo.MessageTypeChannelPinnedMessage
		reply := recapMessage("reply", now.Add(-time.Hour), "human", "reply")
		reply.Type = discordgo.MessageTypeReply
		calls := 0
		got, err := fetchRecapMessages(context.Background(), now, func(ctx context.Context, before string, limit int) ([]*discordgo.Message, error) {
			calls++
			return []*discordgo.Message{bot, webhook, system, reply,
				recapMessage("boundary", now.Add(-24*time.Hour), "human", "keep"),
				recapMessage("old", now.Add(-24*time.Hour-time.Second), "human", "skip")}, nil
		})
		if err != nil || len(got) != 2 || calls != 1 || got[0].ID != "boundary" || got[1].ID != "reply" {
			t.Fatalf("cutoff/filter failed: %+v %v", got, err)
		}
	})
	t.Run("full cutoff page stops pagination", func(t *testing.T) {
		calls := 0
		got, err := fetchRecapMessages(context.Background(), now, func(ctx context.Context, before string, limit int) ([]*discordgo.Message, error) {
			calls++
			page := make([]*discordgo.Message, 50)
			for n := range page {
				page[n] = recapMessage(fmt.Sprint(n), now.Add(-time.Duration(n)*time.Hour), "human", "hello")
			}
			return page, nil
		})
		t.Run("bot-only pages still count toward 100", func(t *testing.T) {
			calls := 0
			got, err := fetchRecapMessages(context.Background(), now, func(ctx context.Context, before string, limit int) ([]*discordgo.Message, error) {
				calls++
				if calls > 2 {
					t.Fatal("fetched more than 100 bot messages")
				}
				page := make([]*discordgo.Message, 50)
				for n := range page {
					page[n] = recapMessage(fmt.Sprint(calls*50+n), now, "bot", "skip")
					page[n].Author.Bot = true
				}
				return page, nil
			})
			if err != nil || len(got) != 0 || calls != 2 {
				t.Fatalf("scan cap failed: calls=%d len=%d err=%v", calls, len(got), err)
			}
		})
		if err != nil || calls != 1 || len(got) != 25 {
			t.Fatalf("did not stop at cutoff, calls=%d len=%d err=%v", calls, len(got), err)
		}
	})
	t.Run("error and cancellation", func(t *testing.T) {
		_, err := fetchRecapMessages(context.Background(), now, func(context.Context, string, int) ([]*discordgo.Message, error) {
			return nil, fmt.Errorf("history unavailable")
		})
		if err == nil {
			t.Fatal("history error ignored")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := fetchRecapMessages(ctx, now, func(context.Context, string, int) ([]*discordgo.Message, error) {
			t.Fatal("canceled fetch invoked")
			return nil, nil
		}); err == nil {
			t.Fatal("cancellation ignored")
		}
	})
}

func TestRecapAuthorizationRefusals(t *testing.T) {
	for _, tt := range []struct{ name, user, guild, channel, want string }{
		{"nonstaff", slashTestParent, slashTestGuild, recapTestChannel, "approved staff"},
		{"wrong guild", slashTestStaff, "other", recapTestChannel, "approved staff"},
		{"DM", slashTestStaff, "", recapTestChannel, "approved staff"},
		{"not allowlisted", slashTestStaff, slashTestGuild, "other", "not enabled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bot := opsTestBot()
			bot.recapChannels = map[string]bool{recapTestChannel: true}
			i := slashTestInteraction("kit", tt.user, tt.guild, "invoking").Interaction
			// A nil session proves no Discord history/channel call was attempted.
			got := bot.handleRecap(nil, i, tt.channel)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("unexpected refusal: %s", got)
			}
		})
	}
	request, err := parseDiscordSlash(discordgo.ApplicationCommandInteractionData{Name: "kit",
		Options: []*discordgo.ApplicationCommandInteractionDataOption{recapOption(recapTestChannel)}})
	if err != nil || request.name != "kit-recap" || request.recapChannel != recapTestChannel ||
		request.flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("bad recap parsing: %+v %v", request, err)
	}
	for _, invalid := range []*discordgo.ApplicationCommandInteractionDataOption{
		recapOption("bad-id"), {Name: "recap", Type: discordgo.ApplicationCommandOptionSubCommand},
		{Name: "recap", Type: discordgo.ApplicationCommandOptionSubCommand, Options: []*discordgo.ApplicationCommandInteractionDataOption{nil}},
	} {
		if _, err := parseDiscordSlash(discordgo.ApplicationCommandInteractionData{Name: "kit",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{invalid}}); err == nil {
			t.Fatal("invalid recap options accepted")
		}
	}
}

func TestRecapEphemeralDeliveryAndPrivacy(t *testing.T) {
	for _, mode := range []string{"summary", "empty", "bot-only", "history-failure", "channel-failure", "wrong-source-guild", "forum", "AI-failure", "long", "ack-failure", "delivery-failure", "refused"} {
		t.Run(mode, func(t *testing.T) {
			s, calls := slashTestSession(t, 200)
			bot := opsTestBot()
			bot.recapChannels = map[string]bool{recapTestChannel: true}
			store := NewInMemorySessionStore()
			aiCalls, historyReads := 0, 0
			bot.aiService = NewAIService(store, func(string) string {
				t.Fatal("recap must not use conversational fallback")
				return ""
			}, providerFunc{name: "capture", fn: func(ctx context.Context, transcript string, history *Session) (string, error) {
				aiCalls++
				if history != nil || requestSystemPrompt(ctx) != recapSystemPrompt {
					t.Fatal("recap used conversational prompt/history")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing AI deadline")
				}
				if strings.Contains(transcript, "PrivateName") || strings.Contains(transcript, slashTestParent) ||
					!strings.Contains(transcript, "Camper 1:") {
					t.Fatalf("raw content reached provider: %s", transcript)
				}
				if mode == "AI-failure" {
					return "", fmt.Errorf("PrivateName error content must not be logged")
				}
				if mode == "long" {
					return strings.Repeat("word ", 400), nil
				}
				return "Minecraft building was discussed. @everyone PrivateName alice@example.com", nil
			}})
			original := s.Client.Transport
			s.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					if _, ok := r.Context().Deadline(); !ok {
						t.Error("Discord recap call missing deadline")
					}
					payload, status := "", 200
					if strings.HasSuffix(r.URL.Path, "/messages") {
						historyReads++
						if r.URL.Query().Get("limit") != "50" {
							t.Error("missing history limit")
						}
						message := recapMessage("one", time.Now().Add(-time.Hour), slashTestParent, "PrivateName likes Minecraft")
						message.Author.Username = "PrivateName"
						if mode == "bot-only" {
							message.Author.Bot = true
						}
						data, err := json.Marshal([]*discordgo.Message{message})
						if err != nil {
							t.Fatal(err)
						}
						payload = string(data)
						if mode == "empty" {
							payload = "[]"
						}
						if mode == "history-failure" {
							status, payload = 403, `{"message":"PrivateName"}`
						}
					} else {
						guild, kind := slashTestGuild, 0
						if mode == "wrong-source-guild" {
							guild = "other"
						}
						if mode == "forum" {
							kind = int(discordgo.ChannelTypeGuildForum)
						}
						payload = fmt.Sprintf(`{"id":%q,"guild_id":%q,"type":%d}`, recapTestChannel, guild, kind)
						if mode == "channel-failure" {
							status, payload = 403, `{"message":"PrivateName"}`
						}
					}
					return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
						Body: io.NopCloser(strings.NewReader(payload))}, nil
				}
				if mode == "ack-failure" && r.Method == http.MethodPost ||
					mode == "delivery-failure" && r.Method == http.MethodPatch {
					return nil, fmt.Errorf("synthetic delivery error")
				}
				return original.RoundTrip(r)
			})
			var logs bytes.Buffer
			oldLog := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(oldLog)
			actor := slashTestStaff
			if mode == "refused" {
				actor = slashTestParent
			}
			bot.onDiscordInteraction(s, slashTestInteraction("kit", actor, slashTestGuild, "invoking", recapOption(recapTestChannel)))
			wantAI := mode == "summary" || mode == "AI-failure" || mode == "long" || mode == "delivery-failure"
			if (aiCalls == 1) != wantAI {
				t.Fatalf("AI calls=%d, mode %s", aiCalls, mode)
			}
			if len(store.sessions) != 0 || strings.Contains(logs.String(), "PrivateName") {
				t.Fatal("recap content persisted/logged")
			}
			if mode == "ack-failure" {
				if len(*calls) != 0 || historyReads != 0 {
					t.Fatal("ack failure attempted history or delivery")
				}
				return
			}
			if mode == "refused" && historyReads != 0 {
				t.Fatal("unauthorized user fetched history")
			}
			if len(*calls) == 0 {
				t.Fatal("missing private acknowledgement")
			}
			var ack discordgo.InteractionResponse
			if err := json.Unmarshal((*calls)[0].body, &ack); err != nil {
				t.Fatal(err)
			}
			if ack.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource ||
				ack.Data.Flags != discordgo.MessageFlagsEphemeral {
				t.Fatal("recap not deferred ephemerally")
			}
			for _, call := range (*calls)[1:] {
				if !strings.Contains(call.path, "/webhooks/") || call.method != http.MethodPatch {
					t.Fatalf("unexpected non-private delivery: %+v", call)
				}
				var reply struct {
					Content         string `json:"content"`
					AllowedMentions struct {
						Parse []string `json:"parse"`
					} `json:"allowed_mentions"`
				}
				if err := json.Unmarshal(call.body, &reply); err != nil {
					t.Fatal(err)
				}
				if len(reply.AllowedMentions.Parse) != 0 || strings.Contains(reply.Content, "@everyone") ||
					strings.Contains(reply.Content, "PrivateName") || strings.Contains(reply.Content, "alice@example.com") {
					t.Fatal("unsafe recap output")
				}
				if mode == "long" && len(strings.Fields(reply.Content)) >= 250 {
					t.Fatal("summary exceeded word cap")
				}
			}
		})
	}
}
