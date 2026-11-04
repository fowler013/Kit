package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func reminderOption(action string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name: "remind", Type: discordgo.ApplicationCommandOptionSubCommandGroup,
		Options: []*discordgo.ApplicationCommandInteractionDataOption{{
			Name: action, Type: discordgo.ApplicationCommandOptionSubCommand, Options: options,
		}},
	}
}

func opsTestBot() *DiscordBot {
	return &DiscordBot{
		commands:  discordCommandConfig{guild: slashTestGuild, staff: map[string]bool{slashTestStaff: true}},
		startedAt: time.Now().Add(-time.Hour), staffChannels: map[string]bool{"staff": true},
		reminders: newReminderStore(),
	}
}

func TestKitStaffCommandSchema(t *testing.T) {
	var kit *discordgo.ApplicationCommand
	for _, command := range discordCommands() {
		if command.Name == "kit" {
			kit = command
		}
	}
	if kit == nil || len(kit.Options) != 3 {
		t.Fatal("expected ask, status and remind at /kit root")
	}
	if kit.Options[0].Name != "ask" || len(kit.Options[0].Options) != 2 ||
		kit.Options[1].Name != "status" || len(kit.Options[1].Options) != 0 ||
		kit.Options[2].Name != "remind" || kit.Options[2].Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatal("invalid /kit option nesting")
	}
	var validate func(*discordgo.ApplicationCommandOption)
	validate = func(option *discordgo.ApplicationCommandOption) {
		for _, child := range option.Options {
			if option.Type == discordgo.ApplicationCommandOptionSubCommandGroup && child.Type != discordgo.ApplicationCommandOptionSubCommand {
				t.Fatal("groups must contain subcommands")
			}
			if option.Type == discordgo.ApplicationCommandOptionSubCommand &&
				(child.Type == discordgo.ApplicationCommandOptionSubCommand || child.Type == discordgo.ApplicationCommandOptionSubCommandGroup) {
				t.Fatal("subcommands must contain scalar options only")
			}
			validate(child)
		}
	}
	for _, option := range kit.Options {
		validate(option)
	}
}

func privateTestChannel() *discordgo.Channel {
	return &discordgo.Channel{ID: "staff", GuildID: slashTestGuild, Type: discordgo.ChannelTypeGuildText,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{ID: slashTestGuild, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
			{ID: slashTestStaff, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel},
		}}
}

func mockReminderChannel(session *discordgo.Session, channel *discordgo.Channel, reads *atomic.Int32) {
	original := session.Client.Transport
	session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/channels/") {
			reads.Add(1)
			body, err := json.Marshal(channel)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
				Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/roles") {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
				Body: io.NopCloser(strings.NewReader(`[{"id":"staff-role","name":"Camp Staff"},{"id":"public-role","name":"Campers"}]`))}, nil
		}
		return original.RoundTrip(r)
	})
}

func TestStaffOpsAuthorizationAndEphemeral(t *testing.T) {
	t.Setenv("CAMP_API_BASE_URL", "")
	for _, command := range []string{"status", "remind"} {
		for _, test := range []struct {
			name, actor, guild, channel, want string
		}{
			{"staff", slashTestStaff, slashTestGuild, "staff", "Kit staff status"},
			{"nonstaff", slashTestParent, slashTestGuild, "staff", "restricted"},
			{"wrong guild", slashTestStaff, "another", "staff", "unavailable"},
			{"DM", slashTestStaff, "", "staff", "restricted"},
		} {
			t.Run(command+"/"+test.name, func(t *testing.T) {
				session, calls := slashTestSession(t, 200)
				bot := opsTestBot()
				defer bot.reminders.stop()
				option := &discordgo.ApplicationCommandInteractionDataOption{Name: "status", Type: discordgo.ApplicationCommandOptionSubCommand}
				want := test.want
				if command == "remind" {
					option = reminderOption("create", slashString("in", "30m"), slashString("message", "Synthetic reminder"))
					if test.name == "staff" {
						want = "scheduled"
					}
				}
				var reads atomic.Int32
				mockReminderChannel(session, privateTestChannel(), &reads)
				bot.onDiscordInteraction(session, slashTestInteraction("kit", test.actor, test.guild, test.channel, option))
				if len(*calls) != 2 || !strings.Contains(string((*calls)[0].body), `"flags":64`) ||
					!strings.Contains(string((*calls)[1].body), want) {
					t.Fatalf("unexpected private response: %+v", *calls)
				}
				if reads.Load() > 0 && (command != "remind" || test.name != "staff") {
					t.Fatal("unauthorized request reached channel verification")
				}
			})
		}
	}
}

func TestReminderParsingLimits(t *testing.T) {
	for _, raw := range []string{"1s", "30m", "2h", "167h59m59s", "168h", "1h30m"} {
		if _, err := parseReminderDelay(raw); err != nil {
			t.Errorf("valid duration %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "0s", "-30m", "999ms", "168h1s", "7d", "30", "1e9h", strings.Repeat("9", 100)} {
		if _, err := parseReminderDelay(raw); err == nil {
			t.Errorf("invalid duration accepted: %s", raw)
		}
	}
	for _, option := range []*discordgo.ApplicationCommandInteractionDataOption{
		reminderOption("create", slashString("in", "30m")),
		reminderOption("create", slashString("in", "30m"), slashString("message", "")),
		reminderOption("create", slashString("in", "30m"), slashString("message", strings.Repeat("😀", 501))),
		reminderOption("create", slashString("in", "30m"), slashString("in", "2h")),
		reminderOption("create", slashString("in", "30m"), slashString("unexpected", "text")),
		reminderOption("list", slashString("visibility", "public")),
		reminderOption("cancel", slashString("id", "-1")),
		reminderOption("unknown"),
		{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "status", Options: []*discordgo.ApplicationCommandInteractionDataOption{slashString("visibility", "public")}},
	} {
		_, err := parseDiscordSlash(discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{option}})
		if err == nil {
			t.Errorf("invalid options accepted: %+v", option)
		}
	}
	option := reminderOption("create", slashString("in", "30m"), slashString("message", strings.Repeat("x", maxReminderMessage)))
	request, err := parseDiscordSlash(discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{option}})
	if err != nil || request.flags != discordgo.MessageFlagsEphemeral || request.reminderDelay != 30*time.Minute {
		t.Fatalf("valid bounded reminder rejected: %+v %v", request, err)
	}
}

func TestReminderCapsOwnershipAndShutdown(t *testing.T) {
	store := newReminderStore()
	defer store.stop()
	deliver := func(staffReminder) { t.Error("canceled reminder delivered") }
	for index := 0; index < maxUserReminders; index++ {
		if _, err := store.create("staff", "guild", "channel", "synthetic", time.Hour, deliver); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.create("staff", "other", "other", "synthetic", time.Hour, deliver); err == nil {
		t.Fatal("cap must apply across all channels and guilds")
	}
	if _, err := store.create("another", "guild", "channel", "synthetic", time.Hour, deliver); err != nil {
		t.Fatal("cap applied to wrong user")
	}
	if len(store.list("staff", "guild", "channel")) != 5 || len(store.list("another", "guild", "other")) != 0 {
		t.Fatal("list crossed user/channel boundaries")
	}
	for _, scope := range [][3]string{{"another", "guild", "channel"}, {"staff", "other", "channel"}, {"staff", "guild", "other"}} {
		if store.cancel("1", scope[0], scope[1], scope[2]) {
			t.Fatal("cancel crossed ownership boundary")
		}
	}
	if !store.cancel("1", "staff", "guild", "channel") || store.cancel("1", "staff", "guild", "channel") {
		t.Fatal("cancel not idempotent")
	}
	if _, err := store.create("staff", "guild", "channel", "synthetic", time.Hour, deliver); err != nil {
		t.Fatal("cancel did not release capacity")
	}
	store.stop()
	if len(store.active) != 0 {
		t.Fatal("shutdown retained active timers")
	}
	if _, err := store.create("staff", "guild", "channel", "synthetic", time.Second, deliver); err == nil {
		t.Fatal("accepted reminder during shutdown")
	}
}

func TestReminderShutdownWaitsForDelivery(t *testing.T) {
	store := newReminderStore()
	started, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	if _, err := store.create("staff", "guild", "channel", "synthetic", time.Millisecond, func(staffReminder) {
		close(started)
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("delivery did not start")
	}
	go func() { store.stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("shutdown failed to wait for in-flight delivery")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
}

func TestReminderChannelPrivacy(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*DiscordBot, *discordgo.Channel, *discordgo.Interaction)
		want bool
	}{
		{"private", func(*DiscordBot, *discordgo.Channel, *discordgo.Interaction) {}, true},
		{"ops channel", func(b *DiscordBot, _ *discordgo.Channel, _ *discordgo.Interaction) {
			b.staffChannels = nil
			b.opsChannel = "staff"
		}, true},
		{"unlisted", func(b *DiscordBot, _ *discordgo.Channel, _ *discordgo.Interaction) { b.staffChannels = nil }, false},
		{"DM", func(_ *DiscordBot, _ *discordgo.Channel, i *discordgo.Interaction) { i.GuildID = "" }, false},
		{"wrong guild", func(_ *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) { c.GuildID = "another" }, false},
		{"public", func(_ *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) { c.PermissionOverwrites = nil }, false},
		{"thread", func(_ *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) {
			c.Type = discordgo.ChannelTypeGuildPrivateThread
		}, false},
		{"nonstaff member", func(_ *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) {
			c.PermissionOverwrites[1].ID = slashTestParent
		}, false},
		{"approved role", func(b *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) {
			b.staffRole = "Camp Staff"
			c.PermissionOverwrites[1].ID, c.PermissionOverwrites[1].Type = "staff-role", discordgo.PermissionOverwriteTypeRole
		}, true},
		{"public role", func(b *DiscordBot, c *discordgo.Channel, _ *discordgo.Interaction) {
			b.staffRole = "Camp Staff"
			c.PermissionOverwrites[1].ID, c.PermissionOverwrites[1].Type = "public-role", discordgo.PermissionOverwriteTypeRole
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			bot, channel := opsTestBot(), privateTestChannel()
			defer bot.reminders.stop()
			i := slashTestInteraction("kit", slashTestStaff, slashTestGuild, "staff").Interaction
			test.edit(bot, channel, i)
			session, _ := slashTestSession(t, 200)
			var reads atomic.Int32
			mockReminderChannel(session, channel, &reads)
			got := bot.privateReminderChannel(context.Background(), session, i.GuildID, i.ChannelID)
			if got != test.want {
				t.Fatalf("privacy check=%v want %v", got, test.want)
			}
		})
	}
	bot := opsTestBot()
	defer bot.reminders.stop()
	session, _ := slashTestSession(t, 403)
	if bot.privateReminderChannel(context.Background(), session, slashTestGuild, "unlisted") {
		t.Fatal("unlisted channel accepted")
	}
	session.Client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("synthetic lookup failure") })
	if bot.privateReminderChannel(context.Background(), session, slashTestGuild, "staff") {
		t.Fatal("lookup failure did not fail closed")
	}
}

func TestReminderDeliveryMentionsAndPrivacyRecheck(t *testing.T) {
	for _, test := range []string{"private", "became public", "send failure", "rate limited", "shutdown"} {
		t.Run(test, func(t *testing.T) {
			bot := opsTestBot()
			defer bot.reminders.stop()
			session, _ := discordgo.New("Bot synthetic")
			var channelReads, posts atomic.Int32
			session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				payload := `{"id":"synthetic"}`
				status := 200
				if r.Method == http.MethodGet {
					channel := privateTestChannel()
					if channelReads.Add(1) > 1 && test == "became public" {
						channel.PermissionOverwrites = nil
					}
					body, err := json.Marshal(channel)
					if err != nil {
						return nil, err
					}
					payload = string(body)
				} else {
					posts.Add(1)
					var message discordgo.MessageSend
					if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
						return nil, err
					}
					if r.URL.Path != "/api/v9/channels/staff/messages" ||
						!strings.Contains(message.Content, "@everyone <@123>") || message.AllowedMentions == nil ||
						len(message.AllowedMentions.Parse) != 0 || len(message.AllowedMentions.Users) != 0 || len(message.AllowedMentions.Roles) != 0 {
						t.Error("unsafe reminder delivery")
					}
					if test == "send failure" {
						status = http.StatusForbidden
					}
					if test == "rate limited" {
						status = http.StatusTooManyRequests
						payload = `{"retry_after":0.001,"global":false}`
					}
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(payload))}, nil
			})
			delay := time.Millisecond
			if test == "shutdown" {
				delay = time.Hour
			}
			i := slashTestInteraction("kit", slashTestStaff, slashTestGuild, "staff").Interaction
			result := bot.handleReminder(session, i, discordSlashRequest{
				reminderAction: "create", reminderDelay: delay, reminderMessage: "@everyone <@123>",
			})
			if !strings.Contains(result, "lost on restart") {
				t.Fatal(result)
			}
			if test == "shutdown" {
				bot.session = session
				_ = bot.Stop()
			} else {
				deadline := time.Now().Add(time.Second)
				for channelReads.Load() < 2 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				bot.reminders.stop()
				if channelReads.Load() != 2 {
					t.Fatal("delivery did not recheck privacy")
				}
			}
			wantPosts := int32(1)
			if test == "shutdown" || test == "became public" {
				wantPosts = 0
			}
			if posts.Load() != wantPosts {
				t.Fatalf("posts=%d want %d", posts.Load(), wantPosts)
			}
		})
	}
}

func TestCampHealthProbe(t *testing.T) {
	for _, test := range []struct {
		status     int
		body, want string
	}{
		{200, `{"status":"ok"}`, "healthy"},
		{404, `secret-body`, "reachable, no healthz"},
		{503, `secret-body`, "unhealthy (HTTP 503)"},
		{200, `{"status":"fail"}`, "invalid health response"},
		{200, `<html>secret-body</html>`, "invalid health response"},
		{302, "", "unhealthy (HTTP 302)"},
	} {
		t.Run(fmt.Sprint(test.status, test.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/healthz" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("health probe used data endpoint or credentials")
				}
				w.Header().Set("Location", "/admin/export-json")
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			if got := campHealth(context.Background(), server.URL); !strings.Contains(got, test.want) || strings.Contains(got, "secret-body") {
				t.Fatal(got)
			}
		})
	}
	if campHealth(context.Background(), "") != "not configured" || campHealth(context.Background(), "file:///secret") != "invalid configuration" ||
		campHealth(context.Background(), "https://user:password@example.com") != "invalid configuration" {
		t.Fatal("invalid health configuration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !strings.Contains(campHealth(ctx, "http://127.0.0.1:1"), "unreachable") {
		t.Fatal("failed probe not reported")
	}
}

func TestStaffStatusRedactsSecretsAndTracksHealth(t *testing.T) {
	t.Setenv("CAMP_API_BASE_URL", "")
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "abcdef0123456789")
	for _, key := range []string{"DISCORD_BOT_TOKEN", "GEMINI_API_KEY", "CAMP_ADMIN_PASSWORD", "DISCORD_ATTENDANCE_STAFF_TOKEN"} {
		t.Setenv(key, "synthetic-secret")
	}
	oldCamp := globalCampClient
	globalCampClient = nil
	t.Cleanup(func() { globalCampClient = oldCamp })
	bot := opsTestBot()
	defer bot.reminders.stop()
	succeed := false
	bot.aiService = NewAIService(nil, nil, providerFunc{name: "synthetic-secret", fn: func(context.Context, string, *Session) (string, error) {
		if succeed {
			return "answer", nil
		}
		return "", fmt.Errorf("synthetic-secret")
	}})
	bot.setRegistration("registered")
	for _, want := range []string{"health unknown", "unavailable (last request)", "healthy (last request)"} {
		if want != "health unknown" {
			succeed = want == "healthy (last request)"
			bot.aiService.Respond(context.Background(), ChatRequest{Message: "synthetic"})
		}
		got := bot.staffStatus(context.Background())
		for _, text := range []string{"uptime: 1h", "commit: abcdef0123456789", want, "attendance enabled: false", "slash registration: registered"} {
			if !strings.Contains(got, text) {
				t.Fatalf("missing %q: %s", text, got)
			}
		}
		if strings.Contains(got, "synthetic-secret") {
			t.Fatal("status disclosed a secret")
		}
	}
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "synthetic-secret")
	if strings.Contains(bot.staffStatus(context.Background()), "synthetic-secret") {
		t.Fatal("unvalidated commit disclosed configuration")
	}
}

func TestStartupSelfCheckOptionalAndFailSoft(t *testing.T) {
	t.Setenv("KIT_OPS_CHANNEL_ID", "")
	t.Setenv("CAMP_API_BASE_URL", "")
	bot, err := NewDiscordBot("synthetic", nil, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	defer bot.reminders.stop()
	session, calls := slashTestSession(t, 200)
	bot.postStartupHealth(session)
	if len(*calls) != 0 {
		t.Fatal("self-check posted while unset")
	}
	bot.opsChannel = "invalid"
	bot.postStartupHealth(session)
	if len(*calls) != 0 {
		t.Fatal("invalid channel did not fail closed")
	}
	bot.opsChannel = "123456789012345680"
	bot.commands.guild = slashTestGuild
	bot.commands.staff = map[string]bool{slashTestStaff: true}
	channel := privateTestChannel()
	channel.ID = bot.opsChannel
	var reads atomic.Int32
	mockReminderChannel(session, channel, &reads)
	bot.setRegistration("registered")
	bot.postStartupHealth(session)
	if len(*calls) != 1 || !strings.Contains(string((*calls)[0].body), "slash registration: registered") {
		t.Fatal("missing startup summary")
	}
	var message discordgo.MessageSend
	if err := json.Unmarshal((*calls)[0].body, &message); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Content, "\n") || message.AllowedMentions == nil || len(message.AllowedMentions.Parse) != 0 {
		t.Fatal("self-check is not a mention-safe summary line")
	}
	failed, failedCalls := slashTestSession(t, 403)
	mockReminderChannel(failed, channel, &reads)
	bot.postStartupHealth(failed)
	if len(*failedCalls) != 1 {
		t.Fatal("failed self-check unexpectedly retried")
	}
	channel.PermissionOverwrites = nil
	blocked, blockedCalls := slashTestSession(t, 200)
	mockReminderChannel(blocked, channel, &reads)
	bot.postStartupHealth(blocked)
	if len(*blockedCalls) != 0 {
		t.Fatal("self-check posted in a public channel")
	}
}

func TestStartupSelfCheckRunsOnceAfterRegistration(t *testing.T) {
	t.Setenv("CAMP_API_BASE_URL", "")
	for _, test := range []struct {
		name, application, want string
		register                bool
		status                  int
	}{
		{"registered", slashTestApp, "registered", true, 200},
		{"disabled", slashTestApp, "disabled", false, 200},
		{"invalid configuration", "invalid", "skipped (invalid application or guild)", true, 200},
		{"registration failure", slashTestApp, "failed", true, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, calls := slashTestSession(t, test.status)
			bot := opsTestBot()
			defer bot.reminders.stop()
			bot.commands.application, bot.commands.register = test.application, test.register
			bot.opsChannel = "123456789012345680"
			channel := privateTestChannel()
			channel.ID = bot.opsChannel
			var reads atomic.Int32
			mockReminderChannel(session, channel, &reads)
			event := &discordgo.Ready{User: &discordgo.User{ID: slashTestApp, Username: "Synthetic"}}
			bot.onReady(session, event)
			bot.onReady(session, event)
			posts := 0
			for _, call := range *calls {
				if call.method == http.MethodPost {
					posts++
					if !strings.Contains(string(call.body), "slash registration: "+test.want) {
						t.Fatalf("self-check before registration result: %s", call.body)
					}
				}
			}
			if posts != 1 {
				t.Fatalf("startup summaries=%d want 1", posts)
			}
		})
	}
}
