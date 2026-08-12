package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const githubModelsEndpoint = "https://models.github.ai/inference/chat/completions"

// GitHubModelsClient wraps the GitHub Models inference API (OpenAI-compatible)
type GitHubModelsClient struct {
	token      string
	model      string
	httpClient *http.Client
}

// NewGitHubModelsClient creates a new GitHub Models client
func NewGitHubModelsClient(token, model string) *GitHubModelsClient {
	if token == "" {
		return nil
	}
	if model == "" {
		model = "openai/gpt-4o-mini" // default model
	}

	return &GitHubModelsClient{
		token:      token,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type ghChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ghChatRequest struct {
	Model       string          `json:"model"`
	Messages    []ghChatMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type ghChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateResponse generates a response using the GitHub Models API
func (g *GitHubModelsClient) GenerateResponse(message string) (string, error) {
	if g == nil || g.token == "" {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	systemPrompt := `You are Kit, a helpful and friendly AI assistant integrated into Slack and Discord. Keep responses under 300 words and be professional but approachable.`

	payload := ghChatRequest{
		Model: g.model,
		Messages: []ghChatMessage{
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubModelsEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		log.Printf("❌ GitHub Models API error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ GitHub Models API returned %d: %.200s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("github models api status %d", resp.StatusCode)
	}

	var parsed ghChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("github models api: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) > 0 && parsed.Choices[0].Message.Content != "" {
		return parsed.Choices[0].Message.Content, nil
	}

	return "", nil
}
