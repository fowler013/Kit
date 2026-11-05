package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// DiscordBot holds the Discord bot configuration
type DiscordBot struct {
	session        *discordgo.Session
	geminiClient   *GeminiClient
	claudeClient   *ClaudeClient
	aiService      *AIService
	startTime      string
	botID          string
	mu             sync.RWMutex
	attendance     *discordAttendance
	commands       discordCommandConfig
	registerOnce   sync.Once
	startedAt      time.Time
	registration   string
	opsChannel     string
	staffChannels  map[string]bool
	staffRole      string
	reminders      *reminderStore
	ticketsChannel string
	tickets        ticketStore
}

// NewDiscordBot creates a new Discord bot instance
func NewDiscordBot(token string, geminiClient *GeminiClient, claudeClient *ClaudeClient, startTime string, aiService *AIService) (*DiscordBot, error) {
	if token == "" {
		return nil, fmt.Errorf("Discord token is required")
	}

	// Create Discord session
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %v", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	bot := &DiscordBot{
		session:        session,
		geminiClient:   geminiClient,
		claudeClient:   claudeClient,
		aiService:      aiService,
		startTime:      startTime,
		commands:       discordCommandsFromEnv(),
		startedAt:      time.Now(),
		opsChannel:     strings.TrimSpace(os.Getenv("KIT_OPS_CHANNEL_ID")),
		staffChannels:  parseIDSet(os.Getenv("CAMP_ALLOWED_CHANNEL_IDS")),
		staffRole:      strings.TrimSpace(os.Getenv("CAMP_ALLOWED_ROLE")),
		reminders:      newReminderStore(),
		ticketsChannel: strings.TrimSpace(os.Getenv("KIT_TICKETS_CHANNEL_ID")),
	}
	attendance, err := attendanceFromEnv()
	if err != nil {
		return nil, fmt.Errorf("invalid private attendance configuration")
	}
	if attendance != nil {
		bot.attendance = &discordAttendance{client: attendance}
	}

	// Add event handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onMessageCreate)
	session.AddHandler(bot.onDiscordInteraction)

	return bot, nil
}

// Start starts the Discord bot
func (d *DiscordBot) Start() error {
	log.Println("🔵 Starting Discord bot...")

	// Open Discord connection
	err := d.session.Open()
	if err != nil {
		return fmt.Errorf("failed to open Discord connection: %v", err)
	}

	log.Println("✅ Discord gateway connection opened")
	return nil
}

// Ready uses the gateway's heartbeat state, which is cleared on disconnect.
func (d *DiscordBot) Ready() bool {
	d.session.RLock()
	connected := d.session.DataReady
	d.session.RUnlock()
	return connected && d.userID() != ""
}

func (d *DiscordBot) userID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.botID
}

// Stop stops the Discord bot
func (d *DiscordBot) Stop() error {
	log.Println("🔵 Stopping Discord bot...")
	if d.reminders != nil {
		d.reminders.stop()
	}
	return d.session.Close()
}

// onReady handles the ready event
func (d *DiscordBot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	if event == nil || event.User == nil {
		return
	}
	log.Printf("🔵 Discord bot logged in as: %s#%s", event.User.Username, event.User.Discriminator)
	d.mu.Lock()
	d.botID = event.User.ID
	d.mu.Unlock()
	d.registerOnce.Do(func() {
		d.registerDiscordCommands(s)
		d.postStartupHealth(s)
	})

	// Set bot status
	err := s.UpdateGameStatus(0, "🤖 Kit AI Assistant")
	if err != nil {
		log.Printf("⚠️  Failed to set Discord status: %v", err)
	}
}

// onMessageCreate handles new Discord messages
func (d *DiscordBot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots (including ourselves)
	if m == nil || m.Message == nil || m.Author == nil || m.Author.Bot || m.WebhookID != "" {
		return
	}

	// Check if bot is mentioned or if it's a DM
	isDM := m.GuildID == ""
	botID := d.userID()
	isMentioned := botID != "" && (strings.Contains(m.Content, "<@"+botID+">") || strings.Contains(m.Content, "<@!"+botID+">"))

	if !isDM && !isMentioned && !isDiscordCommand(m.Content) {
		return
	}

	log.Println("🔵 Discord request received")

	// Process the message
	campRoleOK := func() bool { return d.memberHasCampRole(s, m) }
	campChannelOK := func() bool { return d.campChannelAllowed(s, m) }
	response := d.generateDiscordResponse(m.Content, m.Author.ID, m.ChannelID, campRoleOK, campChannelOK)
	if strings.TrimSpace(response) == "" {
		return
	}

	// Send response, splitting long messages to stay under Discord's limit
	for _, chunk := range splitMessage(response, 1900) {
		if _, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:         chunk,
			AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
		}); err != nil {
			log.Printf("❌ Failed to send Discord message: %v", err)
			return
		}
	}
	log.Printf("✅ Discord response sent successfully")
}

func isDiscordCommand(content string) bool {
	fields := strings.Fields(strings.ToLower(content))
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "!status", "!health", "!help", "!commands", "!version", "!myid", "!links", "!camp", "!kit":
		return true
	default:
		return false
	}
}

// splitMessage splits text into chunks of at most limit characters,
// preferring to break on newlines so formatting stays intact.
func splitMessage(text string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	if len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	for len(text) > limit {
		cut := strings.LastIndex(text[:limit], "\n")
		if cut <= 0 {
			cut = limit
			for cut > 0 && !utf8.RuneStart(text[cut]) {
				cut--
			}
			if cut == 0 {
				_, cut = utf8.DecodeRuneInString(text)
			}
		}
		chunks = append(chunks, strings.TrimRight(text[:cut], "\n"))
		text = strings.TrimLeft(text[cut:], "\n")
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

// campLinksMessage builds the !links response from the camp base URL plus any
// extra links configured via CAMP_EXTRA_LINKS ("Name|URL, Name|URL").
func campLinksMessage() string {
	base := globalCampClient.BaseURL()
	if base == "" {
		base = strings.TrimRight(os.Getenv("CAMP_API_BASE_URL"), "/")
	}

	var b strings.Builder
	b.WriteString("🔗 **Camp Power-Up Links**\n\n")
	if base != "" {
		fmt.Fprintf(&b, "• 🏕️ Website & Registration: %s\n", base)
		fmt.Fprintf(&b, "• 🔐 Admin Dashboard: %s/admin\n", base)
	}
	for _, entry := range strings.Split(os.Getenv("CAMP_EXTRA_LINKS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), "|", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			fmt.Fprintf(&b, "• %s: %s\n", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	if base == "" {
		b.WriteString("_No links configured yet. Set `CAMP_API_BASE_URL` and `CAMP_EXTRA_LINKS` in .env._")
	}
	return strings.TrimRight(b.String(), "\n")
}

// memberHasCampRole reports whether the message author holds the Discord role
// configured to grant camp data access (CAMP_ALLOWED_ROLE).
func (d *DiscordBot) memberHasCampRole(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	roleName := globalCampClient.AllowedRole()
	if roleName == "" || m.GuildID == "" || m.Member == nil {
		return false
	}
	roles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		log.Printf("Camp role lookup failed; access denied: %v", err)
		return false
	}
	roleIDs := make(map[string]string, len(roles))
	for _, r := range roles {
		roleIDs[r.ID] = r.Name
	}
	for _, id := range m.Member.Roles {
		if strings.EqualFold(roleIDs[id], roleName) {
			return true
		}
	}
	return false
}

// campChannelAllowed reports whether registration data may be shown where the
// message was sent: always in DMs, otherwise only in a staff channel listed in
// CAMP_ALLOWED_CHANNEL_IDS or a thread inside one. Lookup failures deny access.
func (d *DiscordBot) campChannelAllowed(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if m.GuildID == "" || globalCampClient.ChannelAllowed(m.ChannelID) {
		return true
	}
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		if channel, err = s.Channel(m.ChannelID); err != nil {
			log.Printf("Camp channel lookup failed; access denied: %v", err)
			return false
		}
	}
	return channel.IsThread() && globalCampClient.ChannelAllowed(channel.ParentID)
}

// generateDiscordResponse generates a response for Discord messages.
// campRoleOK is only called for camp queries from users not already allowlisted.
// campChannelOK is only called for camp queries from authorized users; a nil
// value means the channel is not a staff channel.
func (d *DiscordBot) generateDiscordResponse(content, userID, channelID string, campRoleOK, campChannelOK func() bool) string {
	// Clean the message (remove mentions)
	cleanMessage := d.cleanDiscordMessage(content)
	if fields := strings.Fields(cleanMessage); len(fields) > 0 && strings.EqualFold(fields[0], "!kit") {
		cleanMessage = strings.TrimSpace(cleanMessage[len(fields[0]):])
	}

	// Check for special commands first
	if response := d.handleDiscordCommands(cleanMessage); response != "" {
		return response
	}

	if strings.EqualFold(cleanMessage, "!myid") {
		return fmt.Sprintf("🪪 Your Discord user ID is: `%s`", userID)
	}

	if strings.EqualFold(strings.TrimSpace(cleanMessage), "!links") {
		return campLinksMessage()
	}

	// Camp Power-Up data queries: answered directly, never sent to AI providers
	if isCampQuery(cleanMessage) {
		if globalCampClient == nil {
			return "⚠️ Camp integration is not configured. Ask the operator to configure and verify the Camp Power-Up connection."
		}
		hasCampRole := false
		if !globalCampClient.isAuthorized(userID, false) && campRoleOK != nil {
			hasCampRole = campRoleOK()
		}
		// Even authorized users only get registration data in staff channels or DMs.
		if globalCampClient.isAuthorized(userID, hasCampRole) && (campChannelOK == nil || !campChannelOK()) {
			return campPrivacyNotice
		}
		if response := globalCampClient.HandleQuery(cleanMessage, userID, hasCampRole); response != "" {
			return response
		}
	}

	if d.aiService != nil {
		return d.aiService.Respond(context.Background(), ChatRequest{
			Platform:  "discord",
			UserID:    userID,
			ChannelID: channelID,
			Message:   cleanMessage,
		})
	}

	// Fallback to basic responses
	return d.generateDiscordFallback(cleanMessage)
}

// cleanDiscordMessage removes bot mentions and cleans up the message
func (d *DiscordBot) cleanDiscordMessage(content string) string {
	// Remove bot mentions
	botID := d.userID()
	if botID != "" {
		content = strings.ReplaceAll(content, "<@"+botID+">", "")
		content = strings.ReplaceAll(content, "<@!"+botID+">", "")
	}

	// Clean up extra spaces
	content = strings.TrimSpace(content)

	return content
}

// handleDiscordCommands processes special Discord commands
func (d *DiscordBot) handleDiscordCommands(message string) string {
	cleanMessage := strings.ToLower(strings.TrimSpace(message))

	switch {
	case cleanMessage == "!status" || cleanMessage == "!health":
		aiStatus := "Basic responses only (no AI provider configured)"
		if d.aiService != nil && len(d.aiService.providers) > 0 {
			names := make([]string, 0, len(d.aiService.providers))
			for _, provider := range d.aiService.providers {
				names = append(names, provider.Name())
			}
			aiStatus = "Configured: " + strings.Join(names, ", ") + " (availability checked per request)"
		} else if d.geminiClient != nil {
			aiStatus = "✅ Online (Gemini)"
		} else if d.claudeClient != nil {
			aiStatus = "✅ Online (Claude)"
		}

		return fmt.Sprintf("🤖 **Kit Discord Status**\n"+
			"• Bot Status: ✅ Online and Connected\n"+
			"• AI Engine: %s\n"+
			"• Started: %s\n"+
			"• Platform: Discord\n"+
			"• Ready to help! 🚀", aiStatus, d.startTime)

	case cleanMessage == "!help" || cleanMessage == "!commands":
		return "🤖 **Kit Discord Commands**\n\n" +
			"**Special Commands:**\n" +
			"• `!status` - Check bot health\n" +
			"• `!help` - Show this help message\n" +
			"• `!version` - Show version info\n" +
			"• `!myid` - Show your Discord user ID\n" +
			"• `!links` - Camp Power-Up website links\n" +
			"• `!camp` - Camp registration commands (authorized staff, in staff channels or DMs)\n\n" +
			"**How to use Kit on Discord:**\n" +
			"• Send direct messages for private conversations\n" +
			"• Mention @Kit in servers to get responses\n" +
			"• Use the listed !commands in server channels without a mention\n" +
			"• Ask questions, request help, or just chat!\n\n" +
			"I'm powered by AI and ready to assist! 🧠✨"

	case cleanMessage == "!version":
		return "🤖 **Kit v1.0 - Discord Edition**\n" +
			"• Built with Go + DiscordGo\n" +
			"• Powered by Google Gemini AI\n" +
			"• Multi-platform support (Slack + Discord)\n" +
			"• Open source and ready to help! 🚀"
	}

	return "" // No special command matched
}

// generateDiscordFallback provides fallback responses for Discord
func (d *DiscordBot) generateDiscordFallback(cleanMessage string) string {
	cleanMessage = strings.ToLower(cleanMessage)

	switch {
	case strings.Contains(cleanMessage, "hello") || strings.Contains(cleanMessage, "hi") || strings.Contains(cleanMessage, "hey"):
		return "👋 **Hello there!** I'm Kit, your AI assistant on Discord!\n\n" +
			"I'm powered by AI and ready to help with questions, conversations, or tasks!\n\n" +
			"Try asking me something or type `!help` for available commands. 🤖✨"

	case strings.Contains(cleanMessage, "how are you"):
		return "😊 **I'm doing great on Discord, thanks for asking!**\n\n" +
			"I'm running smoothly and ready to tackle whatever you throw at me!\n\n" +
			"How are you doing today? Anything I can help you with? 🚀"

	case strings.Contains(cleanMessage, "what can you do") || strings.Contains(cleanMessage, "capabilities"):
		return "🚀 **Here's what I can do on Discord:**\n\n" +
			"✅ **Answer complex questions** - I can help with explanations, research, and analysis\n" +
			"✅ **Have intelligent conversations** - Chat about any topic that interests you\n" +
			"✅ **Provide coding help** - Assistance with programming and technical questions\n" +
			"✅ **Creative tasks** - Writing, brainstorming, and creative problem-solving\n" +
			"✅ **General assistance** - Help with work, learning, and daily tasks\n\n" +
			"Just ask me anything! I work on both Discord and Slack! 🧠💫"

	case strings.Contains(cleanMessage, "thank"):
		return "😊 **You're very welcome!** Happy to help anytime on Discord!\n\n" +
			"Feel free to ask me anything else - I'm here and ready to assist! 🤖💙"

	default:
		return fmt.Sprintf("🤖 **Message received on Discord:** \"%s\"\n\n"+
			"I'm Kit, your AI assistant! I can help with questions, conversations, and various tasks.\n\n"+
			"💡 **Try asking me about:**\n"+
			"• Any topic you're curious about\n"+
			"• Help with work or projects\n"+
			"• Creative brainstorming\n"+
			"• Technical questions\n\n"+
			"Or type `!help` to see available commands! ✨", cleanMessage)
	}
}
