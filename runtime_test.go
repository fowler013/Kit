package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

type testGateway struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (g *testGateway) send(value interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.conn.WriteJSON(value)
}

func isolatedBotEnvironment(t *testing.T) string {
	t.Helper()
	for _, key := range []string{
		"DISCORD_BOT_TOKEN", "SLACK_BOT_TOKEN", "SLACK_APP_TOKEN",
		"GEMINI_API_KEY", "CLAUDE_API_KEY", "GITHUB_MODELS_TOKEN",
		"OPENAI_COMPAT_BASE_URL", "OPENAI_COMPAT_MODEL", "OPENAI_COMPAT_API_KEY",
		"CAMP_API_BASE_URL", "CAMP_ADMIN_USERNAME", "CAMP_ADMIN_PASSWORD",
		"CAMP_ALLOWED_DISCORD_IDS", "CAMP_ALLOWED_ROLE", "CAMP_CAPACITY",
		"CAMP_POLL_MINUTES", "CAMP_ALERTS_CHANNEL", "CAMP_STATUS_CHANNEL",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("DISCORD_BOT_TOKEN", "synthetic-test-token")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listener.Close()
	t.Setenv("PORT", port)
	return port
}

// Only the synthetic Discord API and loopback probes are reachable in this test.
func routeTestDiscord(t *testing.T, serverURL string) {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "discord.com" {
			copy := r.Clone(r.Context())
			copy.URL.Scheme = base.Scheme
			copy.URL.Host = base.Host
			return original.RoundTrip(copy)
		}
		if r.URL.Hostname() == "127.0.0.1" || r.URL.Hostname() == "::1" {
			return original.RoundTrip(r)
		}
		return nil, fmt.Errorf("test blocked external host")
	})
	t.Cleanup(func() { http.DefaultTransport = original })
}

func awaitProbe(t *testing.T, port string, ready bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := probeHealth(context.Background(), port)
		if (err == nil) == ready {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("readiness did not become %v", ready)
}

func TestDiscordRuntimeLifecycle(t *testing.T) {
	port := isolatedBotEnvironment(t)
	connections := make(chan *testGateway, 4)
	messages := make(chan discordgo.MessageSend, 4)
	intents := make(chan int, 4)
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/gateway"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"url":%q}`, strings.Replace(serverURL, "http:", "ws:", 1)+"/socket")
		case strings.TrimRight(r.URL.Path, "/") == "/socket":
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			gateway := &testGateway{conn: conn}
			if err := gateway.send(map[string]interface{}{"op": 10, "d": map[string]int{"heartbeat_interval": 100}}); err != nil {
				return
			}
			var identify struct {
				Op int `json:"op"`
				D  struct {
					Intents int `json:"intents"`
				} `json:"d"`
			}
			if err := conn.ReadJSON(&identify); err != nil {
				return
			}
			if identify.Op == 2 {
				intents <- identify.D.Intents
				err = gateway.send(map[string]interface{}{
					"op": 0, "t": "READY", "s": 1,
					"d": map[string]interface{}{
						"v": 9, "session_id": "synthetic-session",
						"resume_gateway_url": strings.Replace(serverURL, "http:", "ws:", 1) + "/socket",
						"user":               map[string]string{"id": "99", "username": "SyntheticKit"},
						"guilds":             []interface{}{},
					},
				})
			} else if identify.Op == 6 {
				err = gateway.send(map[string]interface{}{"op": 0, "t": "RESUMED", "s": 2, "d": map[string]interface{}{}})
			} else {
				t.Errorf("unexpected gateway operation %d", identify.Op)
				return
			}
			if err != nil {
				return
			}
			connections <- gateway
			for {
				var event struct {
					Op int `json:"op"`
				}
				if err := conn.ReadJSON(&event); err != nil {
					return
				}
				if event.Op == 1 {
					if err := gateway.send(map[string]interface{}{"op": 11, "d": nil}); err != nil {
						return
					}
				}
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			var message discordgo.MessageSend
			if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
				t.Error(err)
			}
			messages <- message
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"synthetic-response","channel_id":"123"}`)
		default:
			t.Errorf("unexpected synthetic API request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	defer server.Close()
	routeTestDiscord(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runBot(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("runtime stopped with an error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("runtime failed to stop")
		}
	}()
	var gateway *testGateway
	select {
	case gateway = <-connections:
	case err := <-done:
		t.Fatalf("runtime exited before connecting: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("no synthetic gateway connection")
	}
	awaitProbe(t, port, true)
	if got := <-intents; got != int(discordgo.IntentsGuilds|discordgo.IntentsGuildMessages|discordgo.IntentsDirectMessages|discordgo.IntentsMessageContent) {
		t.Fatalf("wrong identify intents: %d", got)
	}
	if err := gateway.send(map[string]interface{}{
		"op": 0, "t": "MESSAGE_CREATE", "s": 2,
		"d": map[string]interface{}{
			"id": "1", "guild_id": "camp", "channel_id": "123", "content": "!help",
			"author": map[string]interface{}{"id": "456", "username": "SyntheticStaff", "bot": false},
		},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message.Content, "Kit Discord Commands") {
			t.Fatalf("unexpected command response: %s", message.Content)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("unmentioned guild command was not delivered through the gateway")
	}
	// A lost gateway must remove readiness, then recover through discordgo's resume.
	gateway.conn.Close()
	awaitProbe(t, port, false)
	select {
	case <-connections:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway did not reconnect")
	}
	awaitProbe(t, port, true)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("graceful shutdown timed out")
	}
	if err := probeHealth(context.Background(), port); err == nil {
		t.Fatal("health listener remained alive after shutdown")
	}
}

func TestInvalidDiscordCredentialsExit(t *testing.T) {
	isolatedBotEnvironment(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"401: Unauthorized","code":0}`)
	}))
	defer server.Close()
	routeTestDiscord(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runBot(ctx); err == nil || !strings.Contains(err.Error(), "start Discord") {
		t.Fatalf("invalid credentials did not fail startup: %v", err)
	}
}
