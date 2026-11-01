package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// CampMonitor watches the Camp Power-Up site and posts Discord alerts for
// new registrations and website up/down transitions. Channels are resolved
// by name (e.g. "camp-alerts") or by ID, so no manual ID copying is needed.
type CampMonitor struct {
	camp          *CampClient
	session       *discordgo.Session
	alertsChannel string // channel name or ID for registration alerts
	statusChannel string // channel name or ID for website status alerts
	interval      time.Duration

	lastCount int
	haveCount bool
	siteUp    bool
	haveSite  bool
}

// NewCampMonitor creates a monitor. Returns nil when there is nothing to do.
func NewCampMonitor(camp *CampClient, session *discordgo.Session, alertsChannel, statusChannel string, interval time.Duration) *CampMonitor {
	if camp == nil || session == nil {
		return nil
	}
	if alertsChannel == "" && statusChannel == "" {
		return nil
	}
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	return &CampMonitor{
		camp:          camp,
		session:       session,
		alertsChannel: alertsChannel,
		statusChannel: statusChannel,
		interval:      interval,
	}
}

// Start launches the monitoring loop in a background goroutine.
func (m *CampMonitor) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if m == nil {
		close(done)
		return done
	}
	log.Printf("📡 Camp monitor started (every %s; alerts→%q, status→%q)", m.interval, m.alertsChannel, m.statusChannel)
	go func() {
		defer close(done)
		if ctx.Err() != nil {
			return
		}
		m.tick(ctx) // establish baselines immediately
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.tick(ctx)
			}
		}
	}()
	return done
}

func (m *CampMonitor) tick(ctx context.Context) {
	if m.statusChannel != "" {
		m.checkWebsite(ctx)
	}
	if m.alertsChannel != "" && ctx.Err() == nil {
		m.checkRegistrations(ctx)
	}
}

func (m *CampMonitor) checkWebsite(ctx context.Context) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.camp.BaseURL(), nil)
	if err != nil {
		log.Printf("Camp monitor: invalid website request: %v", err)
		return
	}
	resp, err := client.Do(req) // #nosec G704 -- validated operator-configured URL (see validateBaseURL)
	up := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400
	if resp != nil {
		_ = resp.Body.Close()
	}
	if ctx.Err() != nil {
		return
	}

	if !m.haveSite {
		m.haveSite = true
		m.siteUp = up
		if up {
			m.post(ctx, m.statusChannel, fmt.Sprintf("🟢 **Website monitor active** - %s is up. I'll post here if it goes down.", m.camp.BaseURL()))
		} else {
			m.post(ctx, m.statusChannel, fmt.Sprintf("🔴 **Website monitor active** - %s appears to be DOWN right now!", m.camp.BaseURL()))
		}
		return
	}

	if up == m.siteUp {
		return // no transition, stay quiet
	}
	m.siteUp = up
	if up {
		m.post(ctx, m.statusChannel, fmt.Sprintf("🟢 **Website recovered** - %s is back up.", m.camp.BaseURL()))
	} else {
		detail := "no response"
		if err != nil {
			detail = err.Error()
		} else if resp != nil {
			detail = resp.Status
		}
		m.post(ctx, m.statusChannel, fmt.Sprintf("🔴 **Website DOWN** - %s is not responding (%s). Check the Railway dashboard!", m.camp.BaseURL(), detail))
	}
}

func (m *CampMonitor) checkRegistrations(ctx context.Context) {
	count, err := m.camp.registrationCount(ctx)
	if err != nil {
		log.Printf("⚠️  Camp monitor: registration check failed: %v", err)
		return
	}

	if !m.haveCount {
		m.haveCount = true
		m.lastCount = count
		return // baseline only, no alert on startup
	}

	if count == m.lastCount {
		return
	}

	diff := count - m.lastCount
	m.lastCount = count

	capacityNote := ""
	if m.camp.capacity > 0 {
		capacityNote = fmt.Sprintf(" (%d/%d spots filled)", count, m.camp.capacity)
	} else {
		capacityNote = fmt.Sprintf(" (%d total)", count)
	}

	if diff > 0 {
		plural := ""
		if diff > 1 {
			plural = "s"
		}
		m.post(ctx, m.alertsChannel, fmt.Sprintf("🎉 **%d new camper%s registered!**%s\nAsk me `!camp roster` for details.", diff, plural, capacityNote))
	} else {
		m.post(ctx, m.alertsChannel, fmt.Sprintf("📉 **%d registration(s) removed**%s", -diff, capacityNote))
	}
}

// post sends a message to a channel referenced by name or ID.
func (m *CampMonitor) post(ctx context.Context, nameOrID, message string) {
	if ctx.Err() != nil {
		return
	}
	channelID := m.resolveChannel(ctx, nameOrID)
	if channelID == "" {
		log.Println("⚠️  Camp monitor: configured alert channel not found - create it or check the name in .env")
		return
	}
	if _, err := m.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:         message,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}, discordgo.WithContext(ctx)); err != nil {
		log.Printf("❌ Camp monitor: failed to post alert: %v", err)
	}
}

// resolveChannel accepts a raw channel ID or a channel name (with or without
// leading '#') and returns the channel ID, searching all guilds the bot is in.
func (m *CampMonitor) resolveChannel(ctx context.Context, nameOrID string) string {
	nameOrID = strings.TrimPrefix(strings.TrimSpace(nameOrID), "#")
	if nameOrID == "" {
		return ""
	}
	if isAllDigits(nameOrID) {
		return nameOrID // already an ID
	}
	m.session.State.RLock()
	guildIDs := make([]string, 0, len(m.session.State.Guilds))
	for _, guild := range m.session.State.Guilds {
		guildIDs = append(guildIDs, guild.ID)
	}
	m.session.State.RUnlock()
	var channelID string
	for _, guildID := range guildIDs {
		channels, err := m.session.GuildChannels(guildID, discordgo.WithContext(ctx))
		if err != nil {
			log.Printf("Camp monitor: channel lookup failed: %v", err)
			continue
		}
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildText && strings.EqualFold(ch.Name, nameOrID) {
				if channelID != "" {
					log.Println("Camp monitor: channel name is ambiguous; configure a channel ID instead")
					return ""
				}
				channelID = ch.ID
			}
		}
	}
	return channelID
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
