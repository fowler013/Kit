package main

import (
	"context"
	"net"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	var ready atomic.Bool
	server := httptest.NewServer(healthHandler(ready.Load))
	defer server.Close()
	for _, test := range []struct {
		path   string
		ready  bool
		status int
	}{
		{"/healthz", false, 200},
		{"/readyz", false, 503},
		{"/readyz", true, 200},
		{"/readyz", false, 503},
		{"/missing", true, 404},
	} {
		ready.Store(test.ready)
		resp, err := server.Client().Get(server.URL + test.path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != test.status {
			t.Errorf("%s (ready=%v): got %d, want %d", test.path, test.ready, resp.StatusCode, test.status)
		}
	}
}

func TestProbeDoesNotStartBot(t *testing.T) {
	var ready atomic.Bool
	server := httptest.NewServer(healthHandler(ready.Load))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	_, port, _ := net.SplitHostPort(u.Host)
	t.Setenv("PORT", port)
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("SLACK_BOT_TOKEN", "")
	t.Setenv("SLACK_APP_TOKEN", "")
	if err := run(context.Background(), []string{"--health-check"}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("unready probe: %v", err)
	}
	ready.Store(true)
	if err := run(context.Background(), []string{"--health-check"}); err != nil {
		t.Fatalf("probe should succeed without bot credentials: %v", err)
	}
	server.Close()
	if err := run(context.Background(), []string{"--health-check"}); err == nil {
		t.Fatal("probe must fail when no server is listening")
	}
	if err := run(context.Background(), []string{"--unknown"}); err == nil {
		t.Fatal("unknown flag must not start a bot")
	}
}

func TestHealthPort(t *testing.T) {
	for _, test := range []struct {
		port, legacy, want string
	}{
		{"", "", "8080"}, {"", "9090", "9090"}, {"8081", "9090", "8081"},
		{"0", "", ""}, {"-1", "", ""}, {"65536", "", ""}, {"bad", "9090", ""},
	} {
		getenv := func(key string) string {
			if key == "PORT" {
				return test.port
			}
			return test.legacy
		}
		got, err := healthPort(getenv)
		if got != test.want || (err != nil) != (test.want == "") {
			t.Errorf("port=%q legacy=%q: %q, %v", test.port, test.legacy, got, err)
		}
	}
}

func TestRuntimeConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		config map[string]string
		valid  bool
	}{
		{"none", nil, false},
		{"Discord only", map[string]string{"DISCORD_BOT_TOKEN": "test"}, true},
		{"Slack only", map[string]string{"SLACK_BOT_TOKEN": "test", "SLACK_APP_TOKEN": "test"}, true},
		{"partial Slack", map[string]string{"DISCORD_BOT_TOKEN": "test", "SLACK_APP_TOKEN": "test"}, false},
		{"placeholder", map[string]string{"DISCORD_BOT_TOKEN": "your-discord-bot-token-here"}, false},
		{"bad port", map[string]string{"DISCORD_BOT_TOKEN": "test", "PORT": "abc"}, false},
		{"partial AI", map[string]string{"DISCORD_BOT_TOKEN": "test", "OPENAI_COMPAT_MODEL": "test"}, false},
		{"missing camp URL", map[string]string{"DISCORD_BOT_TOKEN": "test", "CAMP_ADMIN_PASSWORD": "test"}, false},
		{"missing camp auth", map[string]string{"DISCORD_BOT_TOKEN": "test", "CAMP_API_BASE_URL": "https://camp.example"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateRuntimeConfig(func(key string) string { return test.config[key] })
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
	camp := map[string]string{
		"DISCORD_BOT_TOKEN": "test", "CAMP_API_BASE_URL": "https://camp.example",
		"CAMP_ADMIN_USERNAME": "staff", "CAMP_ADMIN_PASSWORD": "test",
		"CAMP_ALLOWED_DISCORD_IDS": "123, 456", "CAMP_ALLOWED_CHANNEL_IDS": "555, 777",
	}
	if err := validateRuntimeConfig(func(key string) string { return camp[key] }); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"CAMP_API_BASE_URL":        "https://user:secret@camp.example",
		"CAMP_ALLOWED_DISCORD_IDS": "not-an-id",
		"CAMP_ALLOWED_CHANNEL_IDS": "#moderator-only",
		"CAMP_CAPACITY":            "-1",
		"CAMP_POLL_MINUTES":        "0",
	} {
		old := camp[key]
		camp[key] = value
		if err := validateRuntimeConfig(func(key string) string { return camp[key] }); err == nil {
			t.Errorf("accepted invalid %s", key)
		}
		camp[key] = old
	}
}
