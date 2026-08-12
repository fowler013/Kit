// Command server-setup applies the Discord server template
// (config/discord-server-template.json) to a target guild.
//
// It creates the roles, categories, and channels defined in the template,
// skipping anything that already exists, so it is safe to run repeatedly.
// It never deletes or reorders existing roles or channels.
//
// Usage:
//
//	go run ./cmd/server-setup -guild <GUILD_ID> [-template config/discord-server-template.json] [-dry-run]
//
// Requires DISCORD_BOT_TOKEN in the environment (or .env). The guild ID can
// also be provided via DISCORD_GUILD_ID. The bot needs the Manage Roles and
// Manage Channels permissions in the target guild.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// ServerTemplate is the root of the server template JSON.
type ServerTemplate struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Roles       []RoleTemplate     `json:"roles"`
	Categories  []CategoryTemplate `json:"categories"`
}

// RoleTemplate describes a role to create.
type RoleTemplate struct {
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Hoist       bool   `json:"hoist"`
	Mentionable bool   `json:"mentionable"`
	Permissions string `json:"permissions"` // administrator, moderator, member, bot
}

// CategoryTemplate describes a channel category and its channels.
type CategoryTemplate struct {
	Name     string            `json:"name"`
	Access   string            `json:"access"` // open, read-only, staff-only
	Channels []ChannelTemplate `json:"channels"`
}

// ChannelTemplate describes a single channel within a category.
type ChannelTemplate struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // text, voice
	Topic  string `json:"topic"`
	Access string `json:"access"` // optional override: open, read-only, staff-only
}

// rolePermissions maps template permission presets to Discord permission bits.
func rolePermissions(preset string) int64 {
	switch preset {
	case "administrator":
		return discordgo.PermissionAdministrator
	case "moderator":
		return discordgo.PermissionViewChannel |
			discordgo.PermissionSendMessages |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionVoiceConnect |
			discordgo.PermissionVoiceSpeak |
			discordgo.PermissionKickMembers |
			discordgo.PermissionBanMembers |
			discordgo.PermissionManageMessages |
			discordgo.PermissionModerateMembers |
			discordgo.PermissionManageNicknames
	case "bot":
		return discordgo.PermissionViewChannel |
			discordgo.PermissionSendMessages |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionAttachFiles |
			discordgo.PermissionAddReactions
	default: // member
		return discordgo.PermissionViewChannel |
			discordgo.PermissionSendMessages |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionAttachFiles |
			discordgo.PermissionAddReactions |
			discordgo.PermissionVoiceConnect |
			discordgo.PermissionVoiceSpeak
	}
}

func main() {
	guildFlag := flag.String("guild", "", "Target guild (server) ID; falls back to DISCORD_GUILD_ID")
	templateFlag := flag.String("template", "config/discord-server-template.json", "Path to the server template JSON")
	dryRun := flag.Bool("dry-run", false, "Print what would be created without making changes")
	flag.Parse()

	// .env is optional; environment variables may already be set.
	_ = godotenv.Load()

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ DISCORD_BOT_TOKEN is required (set it in the environment or .env)")
	}

	guildID := *guildFlag
	if guildID == "" {
		guildID = os.Getenv("DISCORD_GUILD_ID")
	}
	if guildID == "" {
		log.Fatal("❌ Guild ID is required (use -guild or set DISCORD_GUILD_ID)")
	}

	tmpl, err := loadTemplate(*templateFlag)
	if err != nil {
		log.Fatalf("❌ Failed to load template: %v", err)
	}
	log.Printf("🔵 Loaded template %q (%d roles, %d categories)", tmpl.Name, len(tmpl.Roles), len(tmpl.Categories))

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("❌ Failed to create Discord session: %v", err)
	}

	if err := apply(session, guildID, tmpl, *dryRun); err != nil {
		log.Fatalf("❌ Setup failed: %v", err)
	}
	log.Println("✅ Server setup complete")
}

// loadTemplate reads and parses the server template JSON.
func loadTemplate(path string) (*ServerTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tmpl ServerTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &tmpl, nil
}

// apply creates the roles, categories, and channels from the template,
// skipping anything that already exists.
func apply(s *discordgo.Session, guildID string, tmpl *ServerTemplate, dryRun bool) error {
	guild, err := s.Guild(guildID)
	if err != nil {
		return fmt.Errorf("fetching guild %s: %w", guildID, err)
	}
	log.Printf("🔵 Target server: %s", guild.Name)
	if dryRun {
		log.Println("🔵 Dry run — no changes will be made")
	}

	roleIDs, err := ensureRoles(s, guildID, tmpl.Roles, dryRun)
	if err != nil {
		return err
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return fmt.Errorf("fetching channels: %w", err)
	}

	for _, cat := range tmpl.Categories {
		catID, err := ensureCategory(s, guildID, cat, channels, roleIDs, dryRun)
		if err != nil {
			return err
		}
		for _, ch := range cat.Channels {
			if err := ensureChannel(s, guildID, ch, cat, catID, channels, roleIDs, dryRun); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureRoles creates any missing roles and returns a map of role name → ID.
func ensureRoles(s *discordgo.Session, guildID string, roles []RoleTemplate, dryRun bool) (map[string]string, error) {
	existing, err := s.GuildRoles(guildID)
	if err != nil {
		return nil, fmt.Errorf("fetching roles: %w", err)
	}
	byName := make(map[string]string, len(existing))
	for _, r := range existing {
		byName[strings.ToLower(r.Name)] = r.ID
	}

	ids := make(map[string]string, len(roles))
	for _, rt := range roles {
		if id, ok := byName[strings.ToLower(rt.Name)]; ok {
			log.Printf("⏭️  Role %q already exists, skipping", rt.Name)
			ids[rt.Name] = id
			continue
		}
		if dryRun {
			log.Printf("📤 Would create role %q", rt.Name)
			continue
		}
		perms := rolePermissions(rt.Permissions)
		role, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{
			Name:        rt.Name,
			Color:       &rt.Color,
			Hoist:       &rt.Hoist,
			Mentionable: &rt.Mentionable,
			Permissions: &perms,
		})
		if err != nil {
			return nil, fmt.Errorf("creating role %q: %w", rt.Name, err)
		}
		log.Printf("✅ Created role %q", rt.Name)
		ids[rt.Name] = role.ID
	}
	return ids, nil
}

// ensureCategory creates a category if missing and returns its channel ID.
func ensureCategory(s *discordgo.Session, guildID string, cat CategoryTemplate, channels []*discordgo.Channel, roleIDs map[string]string, dryRun bool) (string, error) {
	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, cat.Name) {
			log.Printf("⏭️  Category %q already exists, skipping", cat.Name)
			return ch.ID, nil
		}
	}
	if dryRun {
		log.Printf("📤 Would create category %q (access: %s)", cat.Name, cat.Access)
		return "", nil
	}
	created, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                 cat.Name,
		Type:                 discordgo.ChannelTypeGuildCategory,
		PermissionOverwrites: overwritesFor(cat.Access, guildID, roleIDs),
	})
	if err != nil {
		return "", fmt.Errorf("creating category %q: %w", cat.Name, err)
	}
	log.Printf("✅ Created category %q", cat.Name)
	return created.ID, nil
}

// ensureChannel creates a channel inside its category if missing.
func ensureChannel(s *discordgo.Session, guildID string, ch ChannelTemplate, cat CategoryTemplate, catID string, channels []*discordgo.Channel, roleIDs map[string]string, dryRun bool) error {
	chType := discordgo.ChannelTypeGuildText
	if ch.Type == "voice" {
		chType = discordgo.ChannelTypeGuildVoice
	}
	for _, existing := range channels {
		if existing.Type == chType && strings.EqualFold(existing.Name, normalizeChannelName(ch.Name, chType)) {
			log.Printf("⏭️  Channel %q already exists, skipping", ch.Name)
			return nil
		}
	}
	if dryRun {
		log.Printf("📤 Would create %s channel %q in %q", ch.Type, ch.Name, cat.Name)
		return nil
	}

	// A channel-level access setting overrides the category default.
	access := ch.Access
	if access == "" {
		access = cat.Access
	}
	_, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                 ch.Name,
		Type:                 chType,
		Topic:                ch.Topic,
		ParentID:             catID,
		PermissionOverwrites: overwritesFor(access, guildID, roleIDs),
	})
	if err != nil {
		return fmt.Errorf("creating channel %q: %w", ch.Name, err)
	}
	log.Printf("✅ Created %s channel %q in %q", ch.Type, ch.Name, cat.Name)
	return nil
}

// normalizeChannelName mirrors Discord's normalization of text channel names
// (lowercased, spaces become dashes) so existence checks match reliably.
func normalizeChannelName(name string, chType discordgo.ChannelType) string {
	if chType == discordgo.ChannelTypeGuildVoice {
		return name
	}
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

// overwritesFor builds permission overwrites for an access preset.
// The guild ID doubles as the @everyone role ID.
func overwritesFor(access, guildID string, roleIDs map[string]string) []*discordgo.PermissionOverwrite {
	var overwrites []*discordgo.PermissionOverwrite
	staffAllow := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory)

	switch access {
	case "read-only":
		// Everyone can read; only staff and bots can post.
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:    guildID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory,
			Deny:  discordgo.PermissionSendMessages,
		})
		for _, name := range []string{"Admin", "Moderator", "Bot"} {
			if id, ok := roleIDs[name]; ok {
				overwrites = append(overwrites, &discordgo.PermissionOverwrite{
					ID:    id,
					Type:  discordgo.PermissionOverwriteTypeRole,
					Allow: staffAllow,
				})
			}
		}
	case "staff-only":
		// Hidden from everyone; visible to staff, developers, and bots.
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:   guildID,
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionViewChannel,
		})
		for _, name := range []string{"Admin", "Moderator", "Developer", "Bot"} {
			if id, ok := roleIDs[name]; ok {
				overwrites = append(overwrites, &discordgo.PermissionOverwrite{
					ID:    id,
					Type:  discordgo.PermissionOverwriteTypeRole,
					Allow: staffAllow,
				})
			}
		}
	}
	return overwrites
}
