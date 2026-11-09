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
	return c.generate(context.Background(), message, kitSystemPrompt)
}

func (c *ClaudeClient) generate(parent context.Context, message, system string) (string, error) {
	if c == nil {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	// Create the message request
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1000,
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: system,
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
		log.Println("❌ Claude API request failed")
		return "", err
	}

	// Extract text from response
	if len(resp.Content) > 0 {
		if resp.Content[0].Type == "text" {
			return finalizeAIResponse(resp.Content[0].Text, resp.StopReason == anthropic.StopReasonMaxTokens), nil
		}
	}

	return "", nil
}
