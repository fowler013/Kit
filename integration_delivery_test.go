package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

func TestIntegrationTruncatedProviderThroughSlashDelivery(t *testing.T) {
	for _, visibility := range []string{"", "private", "public"} {
		for _, character := range []string{"a", "\U0001F600"} {
			t.Run(visibility+"/"+character, func(t *testing.T) {
				session, calls := slashTestSession(t, http.StatusOK)
				text := strings.Repeat(character, 3900) + " (e.g., Jira,"
				providerCalls := 0
				client := NewOpenAICompatClient("https://example.invalid/v1", "synthetic", "synthetic", "integration")
				client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					providerCalls++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": {"application/json"}},
						Body:       io.NopCloser(strings.NewReader(chatResponseFixture("length", text))),
						Request:    r,
					}, nil
				})}
				service := NewAIService(nil, nil, providerFunc{name: "synthetic", fn: func(_ context.Context, prompt string, _ *Session) (string, error) {
					return client.GenerateResponse(prompt)
				}})
				bot := &DiscordBot{aiService: service, commands: discordCommandConfig{guild: slashTestGuild}}
				bot.onDiscordInteraction(session, slashTestInteraction(
					"kit", slashTestStaff, slashTestGuild, "channel",
					slashAsk("Can you help me create a request form or feedback ticketing system?", visibility),
				))
				if providerCalls != 1 || len(*calls) < 3 {
					t.Fatalf("provider calls=%d, Discord calls=%d; expected chunked response", providerCalls, len(*calls))
				}
				var ack discordgo.InteractionResponse
				if err := json.Unmarshal((*calls)[0].body, &ack); err != nil {
					t.Fatal(err)
				}
				wantFlags := discordgo.MessageFlagsEphemeral
				if visibility == "public" {
					wantFlags = 0
				}
				if ack.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource || ack.Data.Flags != wantFlags {
					t.Fatalf("wrong acknowledgement privacy: %+v", ack)
				}
				var delivered strings.Builder
				for index, call := range (*calls)[1:] {
					var message struct {
						Content         string                            `json:"content"`
						Flags           discordgo.MessageFlags            `json:"flags"`
						AllowedMentions *discordgo.MessageAllowedMentions `json:"allowed_mentions"`
					}
					if err := json.Unmarshal(call.body, &message); err != nil {
						t.Fatal(err)
					}
					if utf16Length(message.Content) > 2000 || !utf8.ValidString(message.Content) {
						t.Fatalf("invalid Discord message at chunk %d", index)
					}
					if message.AllowedMentions == nil || len(message.AllowedMentions.Parse) != 0 {
						t.Fatalf("mentions not suppressed at chunk %d", index)
					}
					if index == 0 {
						if call.method != http.MethodPatch || !strings.HasSuffix(call.path, "/messages/@original") {
							t.Fatalf("first chunk did not edit original: %+v", call)
						}
					} else if call.method != http.MethodPost || message.Flags != wantFlags {
						t.Fatalf("follow-up privacy changed at chunk %d: %+v", index, message)
					}
					delivered.WriteString(message.Content)
				}
				want := text + strings.TrimSpace(aiTruncationNotice)
				if strings.ReplaceAll(delivered.String(), "\n", "") != want {
					t.Fatal("provider text or final truncation notice was lost during actual slash delivery")
				}
			})
		}
	}
}
