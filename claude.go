package main

import (
	"context"
	"log"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeClient wraps the Anthropic client with our specific configuration
type ClaudeClient struct {
	client anthropic.Client
	model  string
}

// NewClaudeClient creates a new Claude client
func NewClaudeClient(apiKey, model string) *ClaudeClient {
	if apiKey == "" {
		return nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	
	return &ClaudeClient{
		client: client,
		model:  model,
	}
}

// GenerateResponse generates a response using Claude AI
func (c *ClaudeClient) GenerateResponse(message string) (string, error) {
	if c == nil {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// System prompt for Kit's personality
	systemPrompt := `You are Kit, a helpful and friendly AI assistant integrated into Slack. Keep responses under 300 words and be professional but approachable.`

	// Create the message request
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1000,
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: systemPrompt,
			},
		},
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock(message),
				},
			},
		},
	})

	if err != nil {
		log.Printf("❌ Claude API error: %v", err)
		return "", err
	}

	// Extract text from response
	if len(resp.Content) > 0 {
		if resp.Content[0].Type == "text" {
			return resp.Content[0].Text, nil
		}
	}

	return "", nil
}
