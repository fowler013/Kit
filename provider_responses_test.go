package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/generative-ai-go/genai"
	googleoption "google.golang.org/api/option"
)

type aiResponseTestClient interface {
	GenerateResponse(string) (string, error)
}

func TestFinalizeAIResponse(t *testing.T) {
	for _, tt := range []struct {
		name      string
		text      string
		truncated bool
		want      string
	}{
		{"complete", "Answer. \n", false, "Answer. \n"},
		{"truncated", "Partial answer", true, "Partial answer" + aiTruncationNotice},
		{"empty complete", "", false, ""},
		{"empty truncated preserves fallback", "", true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := finalizeAIResponse(tt.text, tt.truncated); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderResponseTruncation(t *testing.T) {
	providers := []struct {
		name           string
		truncated      string
		complete       string
		other          string
		response       func(string, string) string
		newClient      func(*testing.T, *http.Client) aiResponseTestClient
		normalizesText bool
		otherStopError bool
	}{
		{
			name: "OpenAI compatible", truncated: "length", complete: "stop", other: "content_filter",
			response: chatResponseFixture,
			newClient: func(t *testing.T, httpClient *http.Client) aiResponseTestClient {
				client := NewOpenAICompatClient("https://example.com/v1", "test-key", "test-model", "test")
				client.httpClient = httpClient
				return client
			},
			normalizesText: true,
		},
		{
			name: "GitHub Models", truncated: "length", complete: "stop", other: "content_filter",
			response: chatResponseFixture,
			newClient: func(t *testing.T, httpClient *http.Client) aiResponseTestClient {
				client := NewGitHubModelsClient("test-token", "test-model")
				client.httpClient = httpClient
				return client
			},
		},
		{
			name: "Claude", truncated: "max_tokens", complete: "end_turn", other: "stop_sequence",
			response: func(reason, text string) string {
				return fmt.Sprintf(`{"id":"test","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":%q}],"stop_reason":%q,"usage":{"input_tokens":1,"output_tokens":1}}`, text, reason)
			},
			newClient: func(t *testing.T, httpClient *http.Client) aiResponseTestClient {
				return &ClaudeClient{
					client: anthropic.NewClient(anthropicoption.WithAPIKey("test-key"), anthropicoption.WithHTTPClient(httpClient)),
					model:  "test-model",
				}
			},
		},
		{
			name: "Gemini", truncated: "MAX_TOKENS", complete: "STOP", other: "SAFETY",
			otherStopError: true,
			response: func(reason, text string) string {
				return fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[{"text":%q}]},"finishReason":%q}]}`, text, reason)
			},
			newClient: func(t *testing.T, httpClient *http.Client) aiResponseTestClient {
				client, err := genai.NewClient(context.Background(), googleoption.WithAPIKey("test-key"), googleoption.WithHTTPClient(httpClient))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := client.Close(); err != nil {
						t.Error(err)
					}
				})
				return &GeminiClient{client: client, model: client.GenerativeModel("test-model")}
			},
		},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				reason  string
				text    string
				notice  bool
				status  int
				body    string
				wantErr bool
			}{
				{name: "truncated", reason: provider.truncated, text: "Answer", notice: true},
				{name: "complete", reason: provider.complete, text: "Answer"},
				{name: "other stop", reason: provider.other, text: "Answer", wantErr: provider.otherStopError},
				{name: "missing stop", text: "Answer"},
				{name: "preserves whitespace", reason: provider.complete, text: " Answer \n"},
				{name: "empty truncated preserves fallback", reason: provider.truncated},
				{name: "HTTP error", status: http.StatusBadRequest, body: `{"error":{"type":"invalid_request_error","message":"synthetic failure","code":400}}`, wantErr: true},
				{name: "malformed JSON", body: `{`, wantErr: true},
				{name: "empty response", body: `{}`},
			} {
				t.Run(tt.name, func(t *testing.T) {
					calls := 0
					httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						status := tt.status
						if status == 0 {
							status = http.StatusOK
						}
						body := tt.body
						if body == "" {
							body = provider.response(tt.reason, tt.text)
						}
						return &http.Response{
							StatusCode: status,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    r,
						}, nil
					})}
					got, err := provider.newClient(t, httpClient).GenerateResponse("test prompt")
					if (err != nil) != tt.wantErr {
						t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
					}
					want := tt.text
					if tt.wantErr {
						want = ""
					}
					if provider.normalizesText {
						want = strings.TrimSpace(want)
					}
					if tt.notice {
						want += aiTruncationNotice
					}
					if got != want {
						t.Fatalf("got %q, want %q", got, want)
					}
					if calls != 1 {
						t.Fatalf("got %d requests, want 1", calls)
					}
				})
			}
		})
	}
}

func TestProviderErrorEnvelope(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"synthetic failure"}}`)),
			Request:    r,
		}, nil
	})}
	github := NewGitHubModelsClient("test-token", "test-model")
	github.httpClient = httpClient
	openai := NewOpenAICompatClient("https://example.com/v1", "test-key", "test-model", "test")
	openai.httpClient = httpClient
	for name, client := range map[string]aiResponseTestClient{"GitHub Models": github, "OpenAI compatible": openai} {
		t.Run(name, func(t *testing.T) {
			got, err := client.GenerateResponse("test")
			if err == nil || !strings.Contains(err.Error(), "synthetic failure") || got != "" {
				t.Fatalf("got %q, %v; want empty response and API error", got, err)
			}
		})
	}
}

func TestTruncationNoticeSurvivesDiscordSplit(t *testing.T) {
	for _, size := range []int{1899, 1900, 1901, 4000} {
		for _, character := range []string{"x", "界"} {
			t.Run(fmt.Sprintf("%d-%s", size, character), func(t *testing.T) {
				response := finalizeAIResponse(strings.Repeat(character, size), true)
				chunks := splitMessage(response, 1900)
				for index, chunk := range chunks {
					if len(chunk) > 1900 || !utf8.ValidString(chunk) {
						t.Fatalf("invalid Discord chunk %d: %d bytes", index, len(chunk))
					}
				}
				rejoined := strings.Join(chunks, "")
				notice := strings.TrimSpace(aiTruncationNotice)
				if strings.Count(rejoined, character) != size || strings.Count(rejoined, notice) != 1 {
					t.Fatal("Discord splitting lost answer text or truncation notice")
				}
				if !strings.HasSuffix(rejoined, notice) {
					t.Fatal("truncation notice is not at the end of the Discord response")
				}
			})
		}
	}
}

func chatResponseFixture(reason, text string) string {
	return fmt.Sprintf(`{"choices":[{"message":{"content":%q},"finish_reason":%q}]}`, text, reason)
}

func TestGeminiResponseWithoutContent(t *testing.T) {
	for _, body := range []string{
		`{"candidates":[]}`,
		`{"candidates":[{"finishReason":"MAX_TOKENS"}]}`,
		`{"candidates":[{"content":{"parts":[]},"finishReason":"MAX_TOKENS"}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			client, err := genai.NewClient(context.Background(), googleoption.WithAPIKey("test-key"), googleoption.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    r,
					}, nil
				}),
			}))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			gemini := &GeminiClient{client: client, model: client.GenerativeModel("test-model")}
			if got, err := gemini.GenerateResponse("test"); err != nil || got != "" {
				t.Fatalf("got %q, %v; want empty fallback", got, err)
			}
		})
	}
}

func TestKitSystemPrompt(t *testing.T) {
	for _, instruction := range []string{
		"video-game guide and companion", "primary purpose is to help campers navigate games",
		"Prioritize Minecraft and Pokemon", "other game questions when you can do so reliably",
		"age-appropriate, encouraging, practical guidance", "help them learn and create",
		"edition/version, platform, or the camper's goal", "Be clear when you are unsure",
		"Never invent game mechanics", "Do not imply you can see a player's game",
		"Do not ask for or repeat private or identifying information",
		"direct the camper to a trusted adult or camp staff",
		"under 300 words", "short sections", "Do not invent Kit features",
		"no built-in database", "in memory and lost on restart",
		"would need to be built", "do not guess commands or features", "official documentation",
	} {
		if !strings.Contains(kitSystemPrompt, instruction) {
			t.Errorf("missing prompt instruction %q", instruction)
		}
	}
}
