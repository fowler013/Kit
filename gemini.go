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
			genai.Text(kitSystemPrompt),
		},
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}
}

// GenerateResponse generates a response using Gemini AI
func (g *GeminiClient) GenerateResponse(message string) (string, error) {
	return g.generate(context.Background(), message, kitSystemPrompt)
}

func (g *GeminiClient) generate(parent context.Context, message, system string) (string, error) {
	if g == nil || g.client == nil || g.model == nil {
		return "", nil // Return empty to use fallback
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	// Copy per request so concurrent help and recap prompts never mutate the shared model.
	model := *g.model
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(system)}}
	resp, err := model.GenerateContent(ctx, genai.Text(message))
	if err != nil {
		log.Println("❌ Gemini API request failed")
		return "", err
	}

	// Extract text from response
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		if textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return finalizeAIResponse(string(textPart), resp.Candidates[0].FinishReason == genai.FinishReasonMaxTokens), nil
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
