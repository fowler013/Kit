package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// DiscordBot holds the Discord bot configuration
type DiscordBot struct {
	session      *discordgo.Session
	geminiClient *GeminiClient
	claudeClient *ClaudeClient
	aiService    *AIService
	startTime    string
	botID        string
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

	bot := &DiscordBot{
		session:      session,
		geminiClient: geminiClient,
		claudeClient: claudeClient,
		aiService:    aiService,
		startTime:    startTime,
	}

	// Add event handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onMessageCreate)

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

	log.Println("✅ Discord bot connected and ready!")
	return nil
}

// Stop stops the Discord bot
func (d *DiscordBot) Stop() error {
	log.Println("🔵 Stopping Discord bot...")
	return d.session.Close()
}

// onReady handles the ready event
func (d *DiscordBot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("🔵 Discord bot logged in as: %s#%s", event.User.Username, event.User.Discriminator)
	d.botID = event.User.ID

	// Set bot status
	err := s.UpdateGameStatus(0, "🤖 Kit AI Assistant")
	if err != nil {
		log.Printf("⚠️  Failed to set Discord status: %v", err)
	}
}

// onMessageCreate handles new Discord messages
func (d *DiscordBot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots (including ourselves)
	if m.Author.Bot {
		return
	}

	// Check if bot is mentioned or if it's a DM
	isDM := m.GuildID == ""
	isMentioned := strings.Contains(m.Content, "<@"+d.botID+">") || strings.Contains(m.Content, "<@!"+d.botID+">")

	if !isDM && !isMentioned {
		return // Only respond to DMs or mentions
	}

	log.Printf("🔵 Discord message received from %s: %s", m.Author.Username, m.Content)

	// Process the message
	hasCampRole := d.memberHasCampRole(s, m)
	response := d.generateDiscordResponse(m.Content, m.Author.ID, m.ChannelID, hasCampRole)

	// Send response, splitting long messages to stay under Discord's limit
	for _, chunk := range splitMessage(response, 1900) {
		if _, err := s.ChannelMessageSend(m.ChannelID, chunk); err != nil {
			log.Printf("❌ Failed to send Discord message: %v", err)
			return
		}
	}
	log.Printf("✅ Discord response sent successfully")
}

// splitMessage splits text into chunks of at most limit characters,
// preferring to break on newlines so formatting stays intact.
func splitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	for len(text) > limit {
		cut := strings.LastIndex(text[:limit], "\n")
		if cut <= 0 {
			cut = limit
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

// generateDiscordResponse generates a response for Discord messages
func (d *DiscordBot) generateDiscordResponse(content, userID, channelID string, hasCampRole bool) string {
	// Clean the message (remove mentions)
	cleanMessage := d.cleanDiscordMessage(content)

	log.Printf("💭 Generating Discord response for: '%s'", cleanMessage)

	// Check for special commands first
	if response := d.handleDiscordCommands(cleanMessage); response != "" {
		return response
	}

	if cleanMessage == "!myid" {
		return fmt.Sprintf("🪪 Your Discord user ID is: `%s`", userID)
	}

	if strings.EqualFold(strings.TrimSpace(cleanMessage), "!links") {
		return campLinksMessage()
	}

	// Camp Power-Up data queries: answered directly, never sent to AI providers
	if globalCampClient != nil {
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
	content = strings.ReplaceAll(content, "<@"+d.botID+">", "")
	content = strings.ReplaceAll(content, "<@!"+d.botID+">", "")

	// Clean up extra spaces
	content = strings.TrimSpace(content)

	return content
}

// handleDiscordCommands processes special Discord commands
func (d *DiscordBot) handleDiscordCommands(message string) string {
	cleanMessage := strings.ToLower(strings.TrimSpace(message))

	switch {
	case cleanMessage == "!status" || cleanMessage == "!health":
		aiStatus := "❌ Offline"
		if d.geminiClient != nil {
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
			"• `!camp` - Camp registration commands (authorized users)\n\n" +
			"**How to use Kit on Discord:**\n" +
			"• Send direct messages for private conversations\n" +
			"• Mention @Kit in servers to get responses\n" +
			"• Ask questions, request help, or just chat!\n\n" +
			"I'm powered by AI and ready to assist! 🧠✨"

	case strings.Contains(cleanMessage, "!version"):
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
