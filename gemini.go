package main

import (
	"context"
	"log"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient wraps the Google Generative AI client with our specific configuration
type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey, modelName string) *GeminiClient {
	if apiKey == "" {
		return nil
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Printf("❌ Failed to create Gemini client: %v", err)
		return nil
	}

	// Create the generative model
	model := client.GenerativeModel(modelName)

	// Configure the model for optimal Slack responses
	model.SetTemperature(0.7)
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(1000)

	// Set system instruction for Kit's personality
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`You are Kit, a helpful and friendly AI assistant integrated into Slack.

Your personality:
- Professional but approachable
- Concise but thorough when needed
- Helpful and solution-oriented
- Use emojis sparingly but appropriately
- Keep responses under 300 words for readability in Slack

Always be ready to:
- Answer questions clearly
- Help with problem-solving
- Provide explanations and guidance
- Assist with work-related tasks
- Maintain a positive, collaborative tone`),
		},
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}
}

// GenerateResponse generates a response using Gemini AI
func (g *GeminiClient) GenerateResponse(message string) (string, error) {
	if g == nil || g.client == nil || g.model == nil {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Generate content
	resp, err := g.model.GenerateContent(ctx, genai.Text(message))
	if err != nil {
		log.Printf("❌ Gemini API error: %v", err)
		return "", err
	}

	// Extract text from response
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return string(textPart), nil
		}
	}

	return "", nil
}

// Close closes the Gemini client
func (g *GeminiClient) Close() error {
	if g != nil && g.client != nil {
		return g.client.Close()
	}
	return nil
}
