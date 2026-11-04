package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const maxReminderDelay = 7 * 24 * time.Hour
const maxReminderMessage = 1000
const maxUserReminders = 5

type staffReminder struct {
	id, user, guild, channel, message string
	due                               time.Time
	timer                             *time.Timer
}

type reminderStore struct {
	mu      sync.Mutex
	active  map[string]*staffReminder
	nextID  uint64
	closed  bool
	sending sync.WaitGroup
}

func newReminderStore() *reminderStore {
	return &reminderStore{active: make(map[string]*staffReminder)}
}

func reminderCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionSubCommandGroup, Name: "remind", Description: "Staff reminders (lost on restart)",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "create", Description: "Remind this approved staff channel once",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "in", Description: "Delay, e.g. 30m or 2h (1 second to 168 hours)", Required: true, MaxLength: 32},
					{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Reminder text (no camper data; mentions are suppressed)", Required: true, MaxLength: maxReminderMessage},
				}},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "list", Description: "List your reminder IDs and times in this channel"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "cancel", Description: "Cancel one of your reminders in this channel",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Reminder ID from create or list", Required: true, MaxLength: 20},
				}},
		},
	}
}

func parseReminderDelay(raw string) (time.Duration, error) {
	delay, err := time.ParseDuration(raw)
	if len(raw) > 32 || err != nil || delay < time.Second || delay > maxReminderDelay {
		return 0, fmt.Errorf("delay must be between 1 second and 168 hours")
	}
	return delay, nil
}

func parseReminderOptions(group *discordgo.ApplicationCommandInteractionDataOption, request *discordSlashRequest) error {
	if group.Type != discordgo.ApplicationCommandOptionSubCommandGroup || len(group.Options) != 1 || group.Options[0] == nil {
		return fmt.Errorf("invalid reminder group")
	}
	action := group.Options[0]
	if action.Type != discordgo.ApplicationCommandOptionSubCommand {
		return fmt.Errorf("invalid reminder action")
	}
	request.reminderAction = action.Name
	expected := map[string][]string{"create": {"in", "message"}, "list": {}, "cancel": {"id"}}
	keys, ok := expected[action.Name]
	if !ok || len(action.Options) != len(keys) {
		return fmt.Errorf("invalid reminder action options")
	}
	values := make(map[string]string)
	for _, option := range action.Options {
		if option == nil || option.Type != discordgo.ApplicationCommandOptionString {
			return fmt.Errorf("invalid reminder option")
		}
		value, ok := option.Value.(string)
		if !ok || strings.TrimSpace(value) == "" || values[option.Name] != "" {
			return fmt.Errorf("invalid reminder value")
		}
		values[option.Name] = strings.TrimSpace(value)
	}
	for _, key := range keys {
		if values[key] == "" {
			return fmt.Errorf("missing reminder option")
		}
	}
	if action.Name == "create" {
		delay, err := parseReminderDelay(values["in"])
		if err != nil {
			return err
		}
		request.reminderDelay = delay
		request.reminderMessage = values["message"]
		if utf16Length(request.reminderMessage) > maxReminderMessage {
			return fmt.Errorf("reminder text too long")
		}
	}
	if action.Name == "cancel" {
		if _, err := strconv.ParseUint(values["id"], 10, 64); err != nil {
			return fmt.Errorf("invalid reminder ID")
		}
		request.reminderID = values["id"]
	}
	return nil
}

func (r *reminderStore) create(user, guild, channel, message string, delay time.Duration, deliver func(staffReminder)) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return "", fmt.Errorf("Reminders are unavailable while Kit is shutting down.")
	}
	count := 0
	for _, reminder := range r.active {
		if reminder.user == user {
			count++
		}
	}
	if count >= maxUserReminders {
		return "", fmt.Errorf("You already have 5 active reminders. Cancel one first.")
	}
	r.nextID++
	id := strconv.FormatUint(r.nextID, 10)
	reminder := &staffReminder{id: id, user: user, guild: guild, channel: channel, message: message, due: time.Now().Add(delay)}
	r.active[id] = reminder
	reminder.timer = time.AfterFunc(delay, func() {
		r.mu.Lock()
		if r.closed || r.active[id] != reminder {
			r.mu.Unlock()
			return
		}
		delete(r.active, id)
		r.sending.Add(1)
		r.mu.Unlock()
		defer r.sending.Done()
		deliver(*reminder)
	})
	return id, nil
}

func (r *reminderStore) list(user, guild, channel string) []staffReminder {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := []staffReminder{}
	for _, reminder := range r.active {
		if reminder.user == user && reminder.guild == guild && reminder.channel == channel {
			result = append(result, *reminder)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].due.Before(result[j].due) })
	return result
}

func (r *reminderStore) cancel(id, user, guild, channel string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	reminder := r.active[id]
	if reminder == nil || reminder.user != user || reminder.guild != guild || reminder.channel != channel {
		return false
	}
	reminder.timer.Stop()
	delete(r.active, id)
	return true
}

func (r *reminderStore) stop() {
	r.mu.Lock()
	r.closed = true
	for id, reminder := range r.active {
		reminder.timer.Stop()
		delete(r.active, id)
	}
	r.mu.Unlock()
	r.sending.Wait()
}

func (d *DiscordBot) reminderChannelAllowed(i *discordgo.Interaction) bool {
	return i.GuildID != "" && i.GuildID == d.commands.guild &&
		(d.staffChannels[i.ChannelID] || (d.opsChannel != "" && i.ChannelID == d.opsChannel))
}

func (d *DiscordBot) privateReminderChannel(ctx context.Context, s *discordgo.Session, guild, channelID string) bool {
	if !d.reminderChannelAllowed(&discordgo.Interaction{GuildID: guild, ChannelID: channelID}) {
		return false
	}
	return d.privateStaffTextChannel(ctx, s, guild, channelID)
}

func (d *DiscordBot) privateStaffTextChannel(ctx context.Context, s *discordgo.Session, guild, channelID string) bool {
	if guild == "" || guild != d.commands.guild || channelID == "" {
		return false
	}
	channel, err := s.Channel(channelID, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
	if err != nil {
		log.Println("Kit reminder channel verification failed; access denied")
		return false
	}
	if channel == nil || channel.ID != channelID || channel.GuildID != guild || channel.Type != discordgo.ChannelTypeGuildText {
		return false
	}
	private := false
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite == nil {
			return false
		}
		if overwrite.Type == discordgo.PermissionOverwriteTypeRole && overwrite.ID == guild {
			private = overwrite.Deny&discordgo.PermissionViewChannel != 0 &&
				overwrite.Allow&discordgo.PermissionViewChannel == 0
		}
	}
	if !private {
		return false
	}
	var roles []*discordgo.Role
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.Allow&discordgo.PermissionViewChannel == 0 {
			continue
		}
		switch overwrite.Type {
		case discordgo.PermissionOverwriteTypeMember:
			if !d.commands.staff[overwrite.ID] && overwrite.ID != d.userID() {
				return false
			}
		case discordgo.PermissionOverwriteTypeRole:
			if d.staffRole == "" {
				return false
			}
			if roles == nil {
				roles, err = s.GuildRoles(guild, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
				if err != nil {
					log.Println("Kit reminder role verification failed; access denied")
					return false
				}
			}
			approved := false
			for _, role := range roles {
				if role != nil && role.ID == overwrite.ID && strings.EqualFold(role.Name, d.staffRole) {
					approved = true
				}
			}
			if !approved {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (d *DiscordBot) handleReminder(s *discordgo.Session, i *discordgo.Interaction, request discordSlashRequest) string {
	if !d.reminderChannelAllowed(i) {
		return "Reminders require an approved staff channel; DMs and unlisted channels are not allowed."
	}
	if d.reminders == nil {
		return "Reminders are unavailable."
	}
	user := interactionUser(i).ID
	switch request.reminderAction {
	case "create":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		private := d.privateReminderChannel(ctx, s, i.GuildID, i.ChannelID)
		cancel()
		if !private {
			return "Reminders require a verified private staff text channel. Ask an operator to check channel permissions."
		}
		id, err := d.reminders.create(user, i.GuildID, i.ChannelID, request.reminderMessage, request.reminderDelay, func(reminder staffReminder) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if !d.privateReminderChannel(ctx, s, reminder.guild, reminder.channel) {
				log.Println("Kit reminder delivery canceled: destination is no longer a verified private staff channel")
				return
			}
			if _, err := s.ChannelMessageSendComplex(reminder.channel, &discordgo.MessageSend{
				Content:         "Kit reminder #" + reminder.id + ": " + reminder.message,
				AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
			}, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false)); err != nil {
				log.Println("Kit reminder delivery failed; not retried")
			}
		})
		if err != nil {
			return err.Error()
		}
		return fmt.Sprintf("Reminder #%s scheduled in %s for this channel. Mentions are suppressed. Reminders are lost on restart.", id, request.reminderDelay)
	case "list":
		lines := []string{"Your reminders in this channel (lost on restart):"}
		for _, reminder := range d.reminders.list(user, i.GuildID, i.ChannelID) {
			lines = append(lines, fmt.Sprintf("#%s - %s", reminder.id, reminder.due.UTC().Format(time.RFC3339)))
		}
		if len(lines) == 1 {
			return "You have no active reminders in this channel. Reminders are lost on restart."
		}
		return strings.Join(lines, "\n")
	case "cancel":
		if d.reminders.cancel(request.reminderID, user, i.GuildID, i.ChannelID) {
			return "Reminder canceled."
		}
		return "No matching active reminder in this channel."
	}
	return "Invalid reminder action."
}
