package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ChatRequest is the shared boundary used by platform adapters and future MCP clients.
type ChatRequest struct {
	Platform  string
	UserID    string
	ChannelID string
	Message   string
}

// Session stores lightweight conversation state for a user/platform/channel.
type Session struct {
	ID        string
	Platform  string
	UserID    string
	ChannelID string
	Messages  []ChatMessage
	UpdatedAt time.Time
}

// ChatMessage stores a single message in the session history.
type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// SessionStore persists chat sessions for adapters and future MCP integrations.
type SessionStore interface {
	GetOrCreate(platform, userID, channelID string) *Session
	Append(session *Session, role, content string)
}

// InMemorySessionStore keeps session state in memory for the current process.
type InMemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{sessions: make(map[string]*Session)}
}

func (s *InMemorySessionStore) key(platform, userID, channelID string) string {
	if channelID == "" {
		return fmt.Sprintf("%s:%s", platform, userID)
	}
	return fmt.Sprintf("%s:%s:%s", platform, userID, channelID)
}

func (s *InMemorySessionStore) GetOrCreate(platform, userID, channelID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(platform, userID, channelID)
	if session, ok := s.sessions[key]; ok {
		session.UpdatedAt = time.Now()
		return session
	}

	session := &Session{
		ID:        fmt.Sprintf("%s-%s-%d", platform, userID, time.Now().UnixNano()),
		Platform:  platform,
		UserID:    userID,
		ChannelID: channelID,
		UpdatedAt: time.Now(),
	}
	s.sessions[key] = session
	return session
}

func (s *InMemorySessionStore) Append(session *Session, role, content string) {
	if session == nil || strings.TrimSpace(content) == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if session.Messages == nil {
		session.Messages = make([]ChatMessage, 0, 8)
	}
	session.Messages = append(session.Messages, ChatMessage{
		Role:      role,
		Content:   strings.TrimSpace(content),
		Timestamp: time.Now(),
	})
	session.UpdatedAt = time.Now()
}

// Provider is the shared contract for AI backends.
type Provider interface {
	Name() string
	Generate(ctx context.Context, message string, session *Session) (string, error)
}

// providerFunc adapts a concrete provider to the shared interface.
type providerFunc struct {
	name string
	fn   func(context.Context, string, *Session) (string, error)
}

func (p providerFunc) Name() string {
	return p.name
}

func (p providerFunc) Generate(ctx context.Context, message string, session *Session) (string, error) {
	if p.fn == nil {
		return "", nil
	}
	return p.fn(ctx, message, session)
}

// AIService owns shared provider routing and the lightweight session store.
type AIService struct {
	providers []Provider
	store     SessionStore
	fallback  func(string) string
}

func NewAIService(store SessionStore, fallback func(string) string, providers ...Provider) *AIService {
	if store == nil {
		store = NewInMemorySessionStore()
	}
	return &AIService{
		providers: providers,
		store:     store,
		fallback:  fallback,
	}
}

func (a *AIService) Register(provider Provider) {
	if provider == nil {
		return
	}
	a.providers = append(a.providers, provider)
}

func (a *AIService) Respond(ctx context.Context, req ChatRequest) string {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return ""
	}

	session := a.store.GetOrCreate(req.Platform, req.UserID, req.ChannelID)
	for _, provider := range a.providers {
		response, err := provider.Generate(ctx, message, session)
		if err == nil && strings.TrimSpace(response) != "" {
			a.store.Append(session, "assistant", response)
			return response
		}
		if err != nil {
			log.Printf("⚠️  %s provider failed: %v", provider.Name(), err)
		}
	}

	if a.fallback != nil {
		return a.fallback(message)
	}
	return ""
}

func newGeminiProvider(client *GeminiClient) Provider {
	return providerFunc{
		name: "gemini",
		fn: func(ctx context.Context, message string, session *Session) (string, error) {
			if client == nil {
				return "", nil
			}
			return client.GenerateResponse(message)
		},
	}
}

func newClaudeProvider(client *ClaudeClient) Provider {
	return providerFunc{
		name: "claude",
		fn: func(ctx context.Context, message string, session *Session) (string, error) {
			if client == nil {
				return "", nil
			}
			return client.GenerateResponse(message)
		},
	}
}

func newGitHubModelsProvider(client *GitHubModelsClient) Provider {
	return providerFunc{
		name: "github-models",
		fn: func(ctx context.Context, message string, session *Session) (string, error) {
			if client == nil {
				return "", nil
			}
			return client.GenerateResponse(message)
		},
	}
}

func newOpenAICompatProvider(client *OpenAICompatClient) Provider {
	name := "openai-compat"
	if client != nil {
		name = client.name
	}
	return providerFunc{
		name: name,
		fn: func(ctx context.Context, message string, session *Session) (string, error) {
			if client == nil {
				return "", nil
			}
			return client.GenerateResponse(message)
		},
	}
}
