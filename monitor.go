package main

import (
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
func (m *CampMonitor) Start() {
	if m == nil {
		return
	}
	log.Printf("📡 Camp monitor started (every %s; alerts→%q, status→%q)", m.interval, m.alertsChannel, m.statusChannel)
	go func() {
		m.tick() // establish baselines immediately
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for range ticker.C {
			m.tick()
		}
	}()
}

func (m *CampMonitor) tick() {
	if m.statusChannel != "" {
		m.checkWebsite()
	}
	if m.alertsChannel != "" {
		m.checkRegistrations()
	}
}

func (m *CampMonitor) checkWebsite() {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(m.camp.BaseURL())
	up := err == nil && resp.StatusCode < 500
	if resp != nil {
		resp.Body.Close()
	}

	if !m.haveSite {
		m.haveSite = true
		m.siteUp = up
		if up {
			m.post(m.statusChannel, fmt.Sprintf("🟢 **Website monitor active** - %s is up. I'll post here if it goes down.", m.camp.BaseURL()))
		} else {
			m.post(m.statusChannel, fmt.Sprintf("🔴 **Website monitor active** - %s appears to be DOWN right now!", m.camp.BaseURL()))
		}
		return
	}

	if up == m.siteUp {
		return // no transition, stay quiet
	}
	m.siteUp = up
	if up {
		m.post(m.statusChannel, fmt.Sprintf("🟢 **Website recovered** - %s is back up.", m.camp.BaseURL()))
	} else {
		detail := "no response"
		if err != nil {
			detail = err.Error()
		} else if resp != nil {
			detail = resp.Status
		}
		m.post(m.statusChannel, fmt.Sprintf("🔴 **Website DOWN** - %s is not responding (%s). Check the Railway dashboard!", m.camp.BaseURL(), detail))
	}
}

func (m *CampMonitor) checkRegistrations() {
	count, err := m.camp.RegistrationCount()
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
		m.post(m.alertsChannel, fmt.Sprintf("🎉 **%d new camper%s registered!**%s\nAsk me `!camp roster` for details.", diff, plural, capacityNote))
	} else {
		m.post(m.alertsChannel, fmt.Sprintf("📉 **%d registration(s) removed**%s", -diff, capacityNote))
	}
}

// post sends a message to a channel referenced by name or ID.
func (m *CampMonitor) post(nameOrID, message string) {
	channelID := m.resolveChannel(nameOrID)
	if channelID == "" {
		log.Printf("⚠️  Camp monitor: channel %q not found - create it or check the name", nameOrID)
		return
	}
	if _, err := m.session.ChannelMessageSend(channelID, message); err != nil {
		log.Printf("❌ Camp monitor: failed to post to %q: %v", nameOrID, err)
	}
}

// resolveChannel accepts a raw channel ID or a channel name (with or without
// leading '#') and returns the channel ID, searching all guilds the bot is in.
func (m *CampMonitor) resolveChannel(nameOrID string) string {
	nameOrID = strings.TrimPrefix(strings.TrimSpace(nameOrID), "#")
	if nameOrID == "" {
		return ""
	}
	if isAllDigits(nameOrID) {
		return nameOrID // already an ID
	}
	for _, guild := range m.session.State.Guilds {
		channels, err := m.session.GuildChannels(guild.ID)
		if err != nil {
			continue
		}
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildText && strings.EqualFold(ch.Name, nameOrID) {
				return ch.ID
			}
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
