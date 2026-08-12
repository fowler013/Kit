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
	"time"
)

// OpenAICompatClient talks to any OpenAI-compatible chat completions API
// (Groq, Mistral, OpenRouter, Cerebras, Ollama, OpenAI itself, etc.)
type OpenAICompatClient struct {
	baseURL    string
	apiKey     string
	model      string
	name       string
	httpClient *http.Client
}

// NewOpenAICompatClient creates a client for an OpenAI-compatible endpoint.
// baseURL should be the API root, e.g. "https://api.groq.com/openai/v1"
// or "http://localhost:11434/v1" for Ollama.
func NewOpenAICompatClient(baseURL, apiKey, model, name string) *OpenAICompatClient {
	validated, err := validateBaseURL(baseURL)
	if err != nil || model == "" {
		return nil
	}
	if name == "" {
		name = "openai-compat"
	}

	return &OpenAICompatClient{
		baseURL:    validated,
		apiKey:     apiKey,
		model:      model,
		name:       name,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

type oaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaChatRequest struct {
	Model       string          `json:"model"`
	Messages    []oaChatMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type oaChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateResponse generates a response using the configured endpoint
func (o *OpenAICompatClient) GenerateResponse(message string) (string, error) {
	if o == nil {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	systemPrompt := `You are Kit, a helpful and friendly AI assistant integrated into Slack and Discord. Keep responses under 300 words and be professional but approachable.`

	payload := oaChatRequest{
		Model: o.model,
		Messages: []oaChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: message},
		},
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := o.baseURL + "/chat/completions"
	// baseURL is validated (scheme+host) in NewOpenAICompatClient; operator config, not user input.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body)) // #nosec G704 -- validated operator-configured URL
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req) // #nosec G704 -- request URL built from validated operator config
	if err != nil {
		log.Printf("❌ OpenAI-compatible API error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ OpenAI-compatible API returned status %d", resp.StatusCode) // #nosec G706 -- StatusCode is an int
		return "", fmt.Errorf("%s api status %d", o.name, resp.StatusCode)
	}

	var parsed oaChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s api: %s", o.name, parsed.Error.Message)
	}

	if len(parsed.Choices) > 0 && parsed.Choices[0].Message.Content != "" {
		return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
	}

	return "", nil
}
