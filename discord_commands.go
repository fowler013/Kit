package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type discordCommandConfig struct {
	guild, application string
	register           bool
	staff              map[string]bool
}

func discordCommandsFromEnv() discordCommandConfig {
	config := discordCommandConfig{
		guild:       strings.TrimSpace(os.Getenv("DISCORD_GUILD_ID")),
		application: strings.TrimSpace(os.Getenv("DISCORD_APPLICATION_ID")),
		staff:       make(map[string]bool),
	}
	if config.guild == "" {
		config.guild = strings.TrimSpace(os.Getenv("DISCORD_ATTENDANCE_GUILD_ID"))
	}
	config.register = config.guild != ""
	if raw := strings.TrimSpace(os.Getenv("DISCORD_REGISTER_COMMANDS")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			log.Println("Discord command registration flag is invalid; registration disabled")
		}
		config.register = err == nil && enabled
	}
	var ids []string
	if raw := strings.TrimSpace(os.Getenv("DISCORD_ATTENDANCE_STAFF_IDS")); raw != "" {
		valid := json.Unmarshal([]byte(raw), &ids) == nil
		for _, id := range ids {
			valid = valid && attendanceID.MatchString(id)
		}
		if valid {
			for _, id := range ids {
				config.staff[id] = true
			}
		} else {
			log.Println("Discord staff allowlist is invalid; slash staff access denied")
		}
	}
	return config
}

func discordCommands() []*discordgo.ApplicationCommand {
	permission := int64(discordgo.PermissionUseSlashCommands)
	dm := false
	commands := []*discordgo.ApplicationCommand{
		{Name: "status", Description: "Check Kit's connection and configured AI providers"},
		{Name: "health", Description: "Alias for /status"},
		{Name: "help", Description: "List the commands available to you"},
		{Name: "commands", Description: "Alias for /help"},
		{Name: "version", Description: "Show Kit's version"},
		{Name: "myid", Description: "Show your Discord user ID privately"},
		{Name: "links", Description: "Show Camp Power-Up website links"},
		{Name: "kit", Description: "Ask Kit, your camp assistant", Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "ask", Description: "Ask Kit a question",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "prompt", Description: "Your question for Kit", Required: true, MaxLength: 4000},
					{Type: discordgo.ApplicationCommandOptionString, Name: "visibility", Description: "Private by default; camp data always stays private",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "private", Value: "private"}, {Name: "public", Value: "public"},
						}},
				}},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "status", Description: "Private operational health for approved staff"},
			reminderCommand(),
		}},
		{Name: "camp", Description: "Private registration queries for authorized staff in approved channels",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "help, stats, roster, unpaid, or capacity"},
			}},
		attendanceCommand(),
		ticketCommand(),
	}
	for _, command := range commands {
		command.Type = discordgo.ChatApplicationCommand
		command.DMPermission = &dm
		command.DefaultMemberPermissions = &permission
	}
	return commands
}

func (d *DiscordBot) registerDiscordCommands(s *discordgo.Session) {
	config := d.commands
	if !config.register {
		d.setRegistration("disabled")
		return
	}
	application := config.application
	if application == "" {
		application = d.userID()
	}
	if !attendanceID.MatchString(application) || !attendanceID.MatchString(config.guild) {
		log.Println("Discord command registration skipped: valid application and guild IDs are required")
		d.setRegistration("skipped (invalid application or guild)")
		return
	}
	// Bulk overwrite is guild-scoped and replaces the same definitions on every startup.
	if _, err := s.ApplicationCommandBulkOverwrite(application, config.guild, discordCommands()); err != nil {
		log.Println("Discord guild command registration failed; bot remains online (check application scope and permissions)")
		d.setRegistration("failed")
		return
	}
	log.Println("Discord guild commands registered successfully")
	d.setRegistration("registered")
}

type discordSlashRequest struct {
	name, prompt                                string
	flags                                       discordgo.MessageFlags
	reminderAction, reminderID, reminderMessage string
	reminderDelay                               time.Duration
}

func parseDiscordSlash(data discordgo.ApplicationCommandInteractionData) (discordSlashRequest, error) {
	request := discordSlashRequest{name: data.Name, flags: discordgo.MessageFlagsEphemeral}
	switch data.Name {
	case "status", "health", "help", "commands", "version", "myid", "links":
		if len(data.Options) != 0 {
			return request, fmt.Errorf("unexpected options")
		}
	case "camp":
		if len(data.Options) > 1 {
			return request, fmt.Errorf("unexpected camp options")
		}
		for _, option := range data.Options {
			if option == nil || option.Name != "query" || option.Type != discordgo.ApplicationCommandOptionString {
				return request, fmt.Errorf("invalid camp query")
			}
			value, ok := option.Value.(string)
			if !ok {
				return request, fmt.Errorf("invalid camp query")
			}
			request.prompt = strings.TrimSpace(value)
		}
	case "kit":
		if len(data.Options) == 1 && data.Options[0] != nil {
			switch data.Options[0].Name {
			case "status":
				if data.Options[0].Type != discordgo.ApplicationCommandOptionSubCommand || len(data.Options[0].Options) != 0 {
					return request, fmt.Errorf("invalid status options")
				}
				request.name = "kit-status"
				return request, nil
			case "remind":
				request.name = "kit-remind"
				err := parseReminderOptions(data.Options[0], &request)
				return request, err
			}
		}
		if len(data.Options) != 1 || data.Options[0] == nil ||
			data.Options[0].Name != "ask" || data.Options[0].Type != discordgo.ApplicationCommandOptionSubCommand {
			return request, fmt.Errorf("expected ask subcommand")
		}
		seen := map[string]bool{}
		for _, option := range data.Options[0].Options {
			if option == nil || seen[option.Name] || option.Type != discordgo.ApplicationCommandOptionString {
				return request, fmt.Errorf("invalid ask option")
			}
			seen[option.Name] = true
			value, ok := option.Value.(string)
			if !ok {
				return request, fmt.Errorf("invalid ask value")
			}
			switch option.Name {
			case "prompt":
				request.prompt = strings.TrimSpace(value)
			case "visibility":
				switch value {
				case "public":
					request.flags = 0
				case "private":
				default:
					return request, fmt.Errorf("invalid visibility")
				}
			default:
				return request, fmt.Errorf("unknown ask option")
			}
		}
		if request.prompt == "" || utf16Length(request.prompt) > 4000 {
			return request, fmt.Errorf("prompt must contain 1-4000 characters")
		}
		// The AI path must never become a public back door to registration data.
		if isCampQuery(request.prompt) {
			request.flags = discordgo.MessageFlagsEphemeral
		}
	default:
		return request, fmt.Errorf("unknown command")
	}
	return request, nil
}

func interactionUser(i *discordgo.Interaction) *discordgo.User {
	if i.GuildID != "" {
		if i.Member != nil {
			return i.Member.User
		}
		return nil
	}
	return i.User
}

func (d *DiscordBot) slashStaff(i *discordgo.Interaction) bool {
	user := interactionUser(i)
	return user != nil && !user.Bot && d.commands.staff[user.ID] &&
		(i.GuildID == "" || (d.commands.guild != "" && i.GuildID == d.commands.guild))
}

func (d *DiscordBot) slashCampAccess(s *discordgo.Session, i *discordgo.Interaction) bool {
	if !d.slashStaff(i) || globalCampClient == nil {
		return false
	}
	message := &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: i.GuildID, ChannelID: i.ChannelID, Member: i.Member,
	}}
	authorized := globalCampClient.isAuthorized(interactionUser(i).ID, false) ||
		globalCampClient.isAuthorized(interactionUser(i).ID, d.memberHasCampRole(s, message))
	return authorized &&
		d.campChannelAllowed(s, message)
}

func (d *DiscordBot) slashHelp(s *discordgo.Session, i *discordgo.Interaction) string {
	lines := []string{
		"**Kit Discord Commands**",
		"`/status` (`/health`) - Connection and AI status",
		"`/help` (`/commands`) - Commands available to you",
		"`/version` - Version information",
		"`/myid` - Your user ID (private)",
		"`/links` - Camp website links",
		"`/kit ask prompt:<text> visibility:<private|public>` - Ask Kit (private by default)",
	}
	if d.slashCampAccess(s, i) {
		lines = append(lines, "`/camp query:<help|stats|roster|unpaid|capacity>` - Private staff registration queries")
	}
	if d.slashStaff(i) {
		lines = append(lines,
			"`/ticket new type:<request|feedback|bug|idea>` - Staff request or feedback form (no personal information)",
			"`/kit status` - Private operational health",
			"`/kit remind create|list|cancel` - One-off reminders in approved staff channels (lost on restart)")
	}
	if d.attendance != nil && d.attendance.client != nil && i.GuildID == d.attendance.client.guild {
		lines = append(lines, "`/attendance parent session:<slug> date:<YYYY-MM-DD>` - Private linked-camper workflow")
		if user := interactionUser(i); user != nil && d.attendance.client.staff[user.ID] {
			lines = append(lines, "`/attendance staff session:<slug> date:<YYYY-MM-DD>` - Private staff attendance")
		}
	}
	lines = append(lines, "Existing `!` commands, `!kit <question>`, @Kit mentions, and DMs still work.")
	return strings.Join(lines, "\n")
}

func (d *DiscordBot) slashResponse(s *discordgo.Session, i *discordgo.Interaction, request discordSlashRequest) string {
	user := interactionUser(i)
	switch request.name {
	case "kit-status", "kit-remind":
		if i.GuildID == "" || !d.slashStaff(i) {
			return "This command is restricted to approved staff."
		}
		if request.name == "kit-status" {
			return d.staffStatus(context.Background())
		}
		return d.handleReminder(s, i, request)
	case "help", "commands":
		return d.slashHelp(s, i)
	case "status", "health", "version":
		return d.handleDiscordCommands("!" + request.name)
	case "myid":
		return fmt.Sprintf("Your Discord user ID is: `%s`", user.ID)
	case "links":
		return campLinksMessage()
	case "camp":
		if !d.slashStaff(i) {
			return "Camp registration data is restricted to approved staff."
		}
		if globalCampClient == nil {
			return "Camp integration is not configured."
		}
		if !d.slashCampAccess(s, i) {
			return "Camp registration access requires an approved staff account and a staff channel or DM."
		}
		return globalCampClient.HandleQuery("!camp "+request.prompt, user.ID, true)
	case "kit":
		if isCampQuery(request.prompt) {
			request.name = "camp"
			return d.slashResponse(s, i, request)
		}
		if d.aiService != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			// Private and public chats must not share provider conversation history.
			scope := "slash-public:" + i.ChannelID
			if request.flags&discordgo.MessageFlagsEphemeral != 0 {
				scope = "slash-private:" + i.ChannelID
			}
			return d.aiService.Respond(ctx, ChatRequest{
				Platform: "discord", UserID: user.ID, ChannelID: scope, Message: request.prompt,
			})
		}
		return d.generateDiscordFallback(request.prompt)
	}
	return "This command is unavailable."
}

func (d *DiscordBot) onDiscordInteraction(s *discordgo.Session, event *discordgo.InteractionCreate) {
	if event == nil || event.Interaction == nil {
		return
	}
	i := event.Interaction
	if isTicketInteraction(i) {
		d.onTicketInteraction(s, i)
		return
	}
	if i.Type != discordgo.InteractionApplicationCommand {
		d.onAttendanceInteraction(s, event)
		return
	}
	data := i.ApplicationCommandData()
	if data.Name == "attendance" {
		d.onAttendanceInteraction(s, event)
		return
	}
	known := false
	for _, command := range discordCommands() {
		known = known || command.Name == data.Name
	}
	if !known {
		return
	}
	request, parseErr := parseDiscordSlash(data)
	user := interactionUser(i)
	authorized := user != nil && !user.Bot &&
		(i.GuildID == "" || d.commands.guild == "" || i.GuildID == d.commands.guild)
	if parseErr != nil || !authorized {
		request.flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: request.flags},
	}); err != nil {
		log.Println("Discord slash acknowledgement failed; no fallback sent")
		return
	}
	response := ""
	switch {
	case !authorized:
		response = "This command is unavailable for this account or server."
	case parseErr != nil:
		response = "Invalid command options. Reopen the command and try again."
	default:
		response = d.slashResponse(s, i, request)
	}
	if strings.TrimSpace(response) == "" {
		response = "Kit could not produce a response. Please try again later."
	}
	for index, chunk := range splitMessage(response, 1900) {
		mentions := &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
		var err error
		if index == 0 {
			_, err = s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &chunk, AllowedMentions: mentions})
		} else {
			_, err = s.FollowupMessageCreate(i, true, &discordgo.WebhookParams{
				Content: chunk, Flags: request.flags, AllowedMentions: mentions,
			})
		}
		if err != nil {
			log.Println("Discord slash response delivery failed; no channel fallback sent")
			return
		}
	}
}
