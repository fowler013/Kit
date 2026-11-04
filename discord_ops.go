package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// buildCommit can be supplied with -ldflags "-X main.buildCommit=<git SHA>".
var buildCommit string

var commitPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)

func currentCommit() string {
	for _, value := range []string{os.Getenv("RAILWAY_GIT_COMMIT_SHA"), buildCommit} {
		if commitPattern.MatchString(value) {
			return value
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && commitPattern.MatchString(setting.Value) {
				return setting.Value
			}
		}
	}
	return "unknown"
}

func (d *DiscordBot) setRegistration(state string) {
	d.mu.Lock()
	d.registration = state
	d.mu.Unlock()
}

func (a *AIService) providerHealth() string {
	if a == nil || len(a.providers) == 0 {
		return "not configured (basic responses only)"
	}
	a.healthMu.RLock()
	defer a.healthMu.RUnlock()
	states := make([]string, 0, len(a.providers))
	for index, provider := range a.providers {
		// Custom provider labels are operator input, not safe diagnostic output.
		name := "OpenAI-compatible"
		switch provider.Name() {
		case "gemini", "claude", "github-models":
			name = provider.Name()
		}
		state := a.health[index]
		if state == "" {
			state = "health unknown (not yet used)"
		}
		states = append(states, name+": configured, "+state)
	}
	return strings.Join(states, "; ")
}

func campHealth(ctx context.Context, base string) string {
	if strings.TrimSpace(base) == "" {
		return "not configured"
	}
	base, err := validateBaseURL(base)
	if err != nil {
		return "invalid configuration"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.User != nil {
		return "invalid configuration"
	}
	client := &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil) // #nosec G704 -- validated operator configuration, not chat input
	if err != nil {
		return "invalid configuration"
	}
	resp, err := client.Do(req) // #nosec G704 -- unauthenticated health probe, redirects disabled
	if err != nil {
		return "unreachable (timeout or connection failure)"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "reachable, no healthz (HTTP 404)"
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("reachable, unhealthy (HTTP %d)", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err != nil || body.Status != "ok" {
		return "reachable, invalid health response"
	}
	return "healthy"
}

func (d *DiscordBot) staffStatus(ctx context.Context) string {
	d.mu.RLock()
	registration := d.registration
	d.mu.RUnlock()
	if registration == "" {
		registration = "not attempted"
	}
	uptime := "unknown"
	if !d.startedAt.IsZero() {
		uptime = time.Since(d.startedAt).Truncate(time.Second).String()
	}
	base := globalCampClient.BaseURL()
	if base == "" {
		base = os.Getenv("CAMP_API_BASE_URL")
	}
	return fmt.Sprintf("Kit staff status | uptime: %s | commit: %s\nAI: %s\nCampPowerUp: %s | attendance enabled: %t | slash registration: %s",
		uptime, currentCommit(), d.aiService.providerHealth(), campHealth(ctx, base),
		d.attendance != nil && d.attendance.client != nil, registration)
}

func (d *DiscordBot) postStartupHealth(s *discordgo.Session) {
	if d.opsChannel == "" {
		return
	}
	if !attendanceID.MatchString(d.opsChannel) {
		log.Println("Kit startup self-check skipped: invalid KIT_OPS_CHANNEL_ID")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !d.privateReminderChannel(ctx, s, d.commands.guild, d.opsChannel) {
		log.Println("Kit startup self-check skipped: destination is not a verified private staff channel")
		return
	}
	content := strings.ReplaceAll(d.staffStatus(ctx), "\n", " | ")
	if _, err := s.ChannelMessageSendComplex(d.opsChannel, &discordgo.MessageSend{
		Content: content, AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false)); err != nil {
		log.Println("Kit startup self-check delivery failed; bot remains online")
	}
}
