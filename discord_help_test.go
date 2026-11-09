package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestLoadServerGuide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.md")
	if err := os.WriteFile(path, []byte("Public guide: use /help."), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		input, want string
		fail        bool
	}{
		{"", "", false},
		{" Inline guide ", "Inline guide", false},
		{"Use /help to find commands", "Use /help to find commands", false},
		{path, "Public guide: use /help.", false},
		{filepath.Dir(path) + string(os.PathSeparator) + "unused" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "guide.md", "Public guide: use /help.", false},
		{filepath.Dir(path), "", true},
		{filepath.Join(t.TempDir(), "missing.md"), "", true},
		{strings.Repeat("x", maxServerGuide), strings.Repeat("x", maxServerGuide), false},
		{strings.Repeat("x", maxServerGuide+1), "", true},
		{string([]byte{0xff}), "", true},
	} {
		got, err := loadServerGuide(tt.input)
		if (err != nil) != tt.fail || got != tt.want {
			t.Errorf("guide length %d: got length %d, err=%v", len(tt.input), len(got), err)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 16001)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadServerGuide(path); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestDiscordHelpConfiguration(t *testing.T) {
	t.Setenv("KIT_SERVER_GUIDE", "Public guide marker")
	t.Setenv("KIT_RECAP_CHANNEL_IDS", recapTestChannel)
	bot, err := NewDiscordBot("synthetic", nil, nil, "", nil)
	if err != nil || bot.serverGuide != "Public guide marker" || !bot.recapChannels[recapTestChannel] {
		t.Fatal("optional help/recap configuration not loaded")
	}
	t.Setenv("KIT_RECAP_CHANNEL_IDS", "not-a-channel-id")
	if _, err := NewDiscordBot("synthetic", nil, nil, "", nil); err == nil {
		t.Fatal("invalid recap allowlist accepted")
	}
}

func TestDiscordHelpHeuristic(t *testing.T) {
	for _, message := range []string{
		"How do I use Discord?", "What can I do in this server?",
		"How do I tag someone?", "How do I use commands in this channel?",
		"What happens if I click this button?", "How do I send a message to a friend?",
		"How do I DM someone?", "notifications on mobile", "reply to a thread",
		"slash commands", "roles and reactions", "friend request", "@mentions",
	} {
		if !isDiscordHelp(message) {
			t.Errorf("missed %q", message)
		}
	}
	for _, message := range []string{"build a Minecraft house", "Pokemon type advantages", "stagger a boss", "the observer block", "tell me a joke"} {
		if isDiscordHelp(message) {
			t.Errorf("unexpected match for %q", message)
		}
	}
}

func TestPublicChannelFilter(t *testing.T) {
	view := int64(discordgo.PermissionViewChannel)
	deny := []*discordgo.PermissionOverwrite{
		{ID: slashTestGuild, Type: discordgo.PermissionOverwriteTypeRole, Deny: view},
		{ID: "staff", Type: discordgo.PermissionOverwriteTypeRole, Allow: view},
	}
	channels := []*discordgo.Channel{
		{ID: "0", GuildID: slashTestGuild, Name: "Games", Type: discordgo.ChannelTypeGuildCategory},
		{ID: "1", GuildID: slashTestGuild, Name: "minecraft", ParentID: "0", Topic: "Build together", Type: discordgo.ChannelTypeGuildText},
		{ID: "2", GuildID: slashTestGuild, Name: "staff-secret", Topic: "never-send-topic", PermissionOverwrites: deny, Type: discordgo.ChannelTypeGuildText},
		{ID: "3", GuildID: slashTestGuild, Name: "announcements", Type: discordgo.ChannelTypeGuildNews},
		{ID: "4", GuildID: slashTestGuild, Name: "forum", Type: discordgo.ChannelTypeGuildForum},
		{ID: "5", GuildID: slashTestGuild, Name: "voice-excluded", Type: discordgo.ChannelTypeGuildVoice},
		{ID: "6", GuildID: slashTestGuild, Name: "thread-excluded", Type: discordgo.ChannelTypeGuildPublicThread},
		{ID: "7", GuildID: "other", Name: "other-guild-excluded"},
	}
	got := publicChannelList(slashTestGuild, []*discordgo.Role{{ID: slashTestGuild, Permissions: view}}, channels)
	for _, want := range []string{"#minecraft", "Build together", "category: Games", "#announcements", "#forum"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	for _, unwanted := range []string{"staff-secret", "never-send-topic", "voice-excluded", "thread-excluded", "other-guild-excluded"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("exposed %q", unwanted)
		}
	}
	if publicChannelList(slashTestGuild, nil, channels) != "" {
		t.Fatal("unknown everyone base must fail closed")
	}
	// Member/other-role grants must not expose a channel without an everyone grant.
	privateBase := []*discordgo.Role{{ID: slashTestGuild}}
	if got := publicChannelList(slashTestGuild, privateBase, channels); strings.Contains(got, "#minecraft") || strings.Contains(got, "staff-secret") {
		t.Fatal("private base role unexpectedly visible")
	}
	allow := &discordgo.Channel{ID: "allowed", GuildID: slashTestGuild, Name: "explicit-public",
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{ID: slashTestGuild, Type: discordgo.PermissionOverwriteTypeRole, Allow: view},
		}}
	if got := publicChannelList(slashTestGuild, privateBase, []*discordgo.Channel{allow}); !strings.Contains(got, "#explicit-public") {
		t.Fatal("explicit everyone allow ignored")
	}
	if !everyoneCanView(slashTestGuild, discordgo.PermissionAdministrator, channels[2]) {
		t.Fatal("administrator base permission must follow Discord semantics")
	}
}

func TestPublicChannelBounds(t *testing.T) {
	channels := []*discordgo.Channel{}
	for n := 0; n < 60; n++ {
		channels = append(channels, &discordgo.Channel{
			ID: fmt.Sprintf("%03d", n), GuildID: slashTestGuild, Name: fmt.Sprintf("room-%d", n),
			Topic: strings.Repeat("x", 500), Type: discordgo.ChannelTypeGuildText,
		})
	}
	got := publicChannelList(slashTestGuild, []*discordgo.Role{{ID: slashTestGuild, Permissions: discordgo.PermissionViewChannel}}, channels)
	if strings.Count(got, "#room-") != 40 || strings.Contains(got, strings.Repeat("x", 164)) {
		t.Fatal("public channel count/topic cap not applied")
	}
}

func TestServerContextGatingAndTTL(t *testing.T) {
	s, calls := slashTestSession(t, 200)
	if err := s.State.GuildAdd(&discordgo.Guild{
		ID:       slashTestGuild,
		Roles:    []*discordgo.Role{{ID: slashTestGuild, Permissions: discordgo.PermissionViewChannel}},
		Channels: []*discordgo.Channel{{ID: "public", GuildID: slashTestGuild, Name: "games"}},
	}); err != nil {
		t.Fatal(err)
	}
	bot := opsTestBot()
	bot.serverGuide = "public-guide-marker"
	for _, tt := range []struct {
		guild, message string
		want           bool
	}{
		{slashTestGuild, "how do I tag someone?", true},
		{slashTestGuild, "Minecraft crafting tips", false},
		{"", "how do I tag someone?", false},
		{"other", "Discord help", false},
	} {
		got := bot.discordServerContext(context.Background(), s, tt.guild, tt.message)
		if strings.Contains(got, "public-guide-marker") != tt.want || strings.Contains(got, "#games") != tt.want {
			t.Errorf("guild %q message %q: wrong context %q", tt.guild, tt.message, got)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("complete gateway state should avoid REST")
	}
	if err := s.State.ChannelAdd(&discordgo.Channel{ID: "public", GuildID: slashTestGuild, Name: "changed"}); err != nil {
		t.Fatal(err)
	}
	if got := bot.discordServerContext(context.Background(), s, slashTestGuild, "Discord"); !strings.Contains(got, "#games") {
		t.Fatal("cache not reused")
	}
	bot.publicChannels.expires = time.Now().Add(-time.Second)
	if got := bot.discordServerContext(context.Background(), s, slashTestGuild, "Discord"); !strings.Contains(got, "#changed") {
		t.Fatal("expired cache not refreshed")
	}
}

func TestPublicReferenceRESTFallbackAndFailure(t *testing.T) {
	s, _ := slashTestSession(t, 200)
	reads := 0
	s.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		reads++
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("REST request missing deadline")
		}
		payload := `[{"id":"` + slashTestGuild + `","permissions":"1024"}]`
		if strings.HasSuffix(r.URL.Path, "/channels") {
			payload = `[{"id":"public","guild_id":"` + slashTestGuild + `","name":"rest-public","type":0}]`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
	})
	// Existing state lacks the everyone role; force channels missing as well.
	s.State = discordgo.NewState()
	if err := s.State.GuildAdd(&discordgo.Guild{ID: slashTestGuild}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bot := opsTestBot()
	if got := bot.discordServerContext(ctx, s, slashTestGuild, "Discord"); !strings.Contains(got, "#rest-public") || reads != 2 {
		t.Fatalf("REST fallback: %q, reads %d", got, reads)
	}
	bot.publicChannels.expires = time.Time{}
	s.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("unavailable") })
	if got := bot.discordServerContext(ctx, s, slashTestGuild, "Discord"); !strings.Contains(got, "unavailable") || strings.Contains(got, "rest-public") {
		t.Fatal("failed refresh should not expose stale reference")
	}
}

func TestDiscordContextReachesPromptOnlyWhenRelevant(t *testing.T) {
	s, _ := slashTestSession(t, 200)
	bot := opsTestBot()
	bot.session = s
	bot.serverGuide = "guide-marker"
	bot.publicChannels = publicChannelCache{guild: slashTestGuild, text: "#public-marker", expires: time.Now().Add(time.Hour)}
	bot.aiService = NewAIService(nil, nil, providerFunc{name: "capture", fn: func(ctx context.Context, message string, session *Session) (string, error) {
		if strings.Contains(requestSystemPrompt(ctx), "guide-marker") != isDiscordHelp(message) {
			t.Errorf("unexpected provider prompt for %q", message)
		}
		return "answer", nil
	}})
	for _, message := range []string{"how do I tag someone?", "build a Minecraft house"} {
		bot.generateDiscordResponse("!kit "+message, slashTestParent, "public", nil, nil, slashTestGuild)
		bot.slashResponse(s, slashTestInteraction("kit", slashTestParent, slashTestGuild, "public").Interaction,
			discordSlashRequest{name: "kit", prompt: message, flags: discordgo.MessageFlagsEphemeral})
	}
	// A later request must not inherit another request's system prompt or reference.
	if got := requestSystemPrompt(context.Background()); got != kitSystemPrompt {
		t.Fatal("request context leaked globally")
	}
}

func TestKitHelpAlias(t *testing.T) {
	request, err := parseDiscordSlash(discordgo.ApplicationCommandInteractionData{Name: "kit",
		Options: []*discordgo.ApplicationCommandInteractionDataOption{{Name: "help", Type: discordgo.ApplicationCommandOptionSubCommand}}})
	if err != nil || request.name != "help" {
		t.Fatal("missing /kit help alias")
	}
	if got := opsTestBot().slashResponse(nil, slashTestInteraction("kit", slashTestParent, slashTestGuild, "public").Interaction, request); !strings.Contains(got, "beginner Discord") {
		t.Fatal("help alias missing Discord help")
	}
}
