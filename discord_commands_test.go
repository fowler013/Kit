package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const slashTestGuild = "1537008248048328726"
const slashTestApp = "1537124728131817603"
const slashTestStaff = "123456789012345678"
const slashTestParent = "123456789012345679"

type slashRESTCall struct {
	method, path string
	body         []byte
}

func slashTestSession(t *testing.T, status int) (*discordgo.Session, *[]slashRESTCall) {
	t.Helper()
	session, err := discordgo.New("Bot synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID: slashTestGuild, Channels: []*discordgo.Channel{{ID: "public", GuildID: slashTestGuild}},
	}); err != nil {
		t.Fatal(err)
	}
	calls := []slashRESTCall{}
	session.ShouldRetryOnRateLimit = false
	session.MaxRestRetries = 0
	session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		calls = append(calls, slashRESTCall{r.Method, r.URL.Path, body})
		payload := `{"id":"response","content":"ok"}`
		if r.Method == http.MethodPut {
			payload = `[]`
		}
		return &http.Response{StatusCode: status,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(strings.NewReader(payload)),
		}, nil
	})
	return session, &calls
}

func slashTestInteraction(name, actor, guild, channel string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	i := &discordgo.Interaction{
		ID: "interaction", AppID: slashTestApp, Token: "synthetic",
		Type: discordgo.InteractionApplicationCommand, GuildID: guild, ChannelID: channel,
		Data: discordgo.ApplicationCommandInteractionData{Name: name, Options: options},
	}
	if guild == "" {
		i.User = &discordgo.User{ID: actor}
	} else {
		i.Member = &discordgo.Member{User: &discordgo.User{ID: actor}}
	}
	return &discordgo.InteractionCreate{Interaction: i}
}

func slashString(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value,
	}
}

func slashAsk(prompt, visibility string) *discordgo.ApplicationCommandInteractionDataOption {
	options := []*discordgo.ApplicationCommandInteractionDataOption{slashString("prompt", prompt)}
	if visibility != "" {
		options = append(options, slashString("visibility", visibility))
	}
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name: "ask", Type: discordgo.ApplicationCommandOptionSubCommand, Options: options,
	}
}

func TestSlashCommandDefinitions(t *testing.T) {
	names := map[string]bool{}
	validName := regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	check := func(name, description string) {
		t.Helper()
		if !validName.MatchString(name) || utf16Length(description) == 0 || utf16Length(description) > 100 {
			t.Fatalf("invalid definition: %q %q", name, description)
		}
	}
	var checkOptions func([]*discordgo.ApplicationCommandOption)
	checkOptions = func(options []*discordgo.ApplicationCommandOption) {
		for _, option := range options {
			check(option.Name, option.Description)
			checkOptions(option.Options)
		}
	}
	for _, command := range discordCommands() {
		t.Run(command.Name, func(t *testing.T) {
			check(command.Name, command.Description)
			checkOptions(command.Options)
			if names[command.Name] {
				t.Fatal("duplicate command")
			}
			names[command.Name] = true
			if command.Type != discordgo.ChatApplicationCommand || command.DMPermission == nil || *command.DMPermission ||
				command.DefaultMemberPermissions == nil || *command.DefaultMemberPermissions != discordgo.PermissionUseSlashCommands {
				t.Fatal("unexpected DM or default member permissions")
			}
		})
	}
	if len(names) != 10 {
		t.Fatalf("got %d commands, want 10", len(names))
	}
	kit := discordCommands()[7].Options[0].Options
	if kit[0].Name != "prompt" || !kit[0].Required || kit[0].MaxLength != 4000 ||
		kit[1].Name != "visibility" || kit[1].Required ||
		kit[1].Choices[0].Value != "private" || kit[1].Choices[1].Value != "public" {
		t.Fatal("unexpected /kit ask schema")
	}
}

func TestSlashRoutingAndPrivacy(t *testing.T) {
	oldCamp := globalCampClient
	globalCampClient = nil
	t.Cleanup(func() { globalCampClient = oldCamp })
	for _, test := range []struct {
		name, visibility, prompt, want string
		public, ai                     bool
	}{
		{name: "status", want: "Kit Discord Status"},
		{name: "health", want: "Kit Discord Status"},
		{name: "help", want: "Kit Discord Commands"},
		{name: "commands", want: "Kit Discord Commands"},
		{name: "version", want: "Kit v1.0"},
		{name: "myid", want: slashTestParent},
		{name: "links", want: "Camp Power-Up Links"},
		{name: "camp", want: "restricted"},
		{name: "kit", prompt: "explain circuits", want: "provider answer", ai: true},
		{name: "kit", prompt: "explain circuits", visibility: "private", want: "provider answer", ai: true},
		{name: "kit", prompt: "explain circuits", visibility: "public", want: "provider answer", public: true, ai: true},
		{name: "kit", prompt: "who registered for camp?", visibility: "public", want: "restricted"},
		{name: "kit", prompt: "!camp roster", visibility: "public", want: "restricted"},
		{name: "kit", prompt: " ", want: "Invalid command"},
		{name: "kit", prompt: "hello", visibility: "invalid", want: "Invalid command"},
	} {
		t.Run(test.name+"/"+test.visibility+"/"+test.prompt, func(t *testing.T) {
			session, calls := slashTestSession(t, 200)
			aiCalls := 0
			service := NewAIService(nil, nil, providerFunc{name: "synthetic", fn: func(_ context.Context, prompt string, history *Session) (string, error) {
				aiCalls++
				if len(*calls) != 1 {
					t.Fatal("provider called before acknowledgement")
				}
				prefix := "slash-private:"
				if test.public {
					prefix = "slash-public:"
				}
				if prompt != test.prompt || !strings.HasPrefix(history.ChannelID, prefix) {
					t.Fatal("wrong prompt or conversation visibility scope")
				}
				return "provider answer", nil
			}})
			bot := &DiscordBot{aiService: service, commands: discordCommandConfig{guild: slashTestGuild}}
			var options []*discordgo.ApplicationCommandInteractionDataOption
			if test.name == "kit" {
				options = append(options, slashAsk(test.prompt, test.visibility))
			}
			bot.onDiscordInteraction(session, slashTestInteraction(test.name, slashTestParent, slashTestGuild, "channel", options...))
			if len(*calls) != 2 {
				t.Fatalf("expected defer and edit, got %+v", *calls)
			}
			var ack discordgo.InteractionResponse
			if err := json.Unmarshal((*calls)[0].body, &ack); err != nil {
				t.Fatal(err)
			}
			wantFlags := discordgo.MessageFlagsEphemeral
			if test.public {
				wantFlags = 0
			}
			if ack.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource || ack.Data.Flags != wantFlags {
				t.Fatalf("unexpected acknowledgement: %+v", ack)
			}
			var edit discordgo.WebhookEdit
			if err := json.Unmarshal((*calls)[1].body, &edit); err != nil {
				t.Fatal(err)
			}
			if (*calls)[1].method != http.MethodPatch || !strings.HasSuffix((*calls)[1].path, "/messages/@original") ||
				edit.Content == nil || !strings.Contains(*edit.Content, test.want) ||
				edit.AllowedMentions == nil || len(edit.AllowedMentions.Parse) != 0 {
				t.Fatalf("unexpected response: %s", (*calls)[1].body)
			}
			if (aiCalls == 1) != test.ai {
				t.Fatal("incorrect AI routing")
			}
		})
	}
}

func TestSlashMalformedRequestsAndIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*discordgo.InteractionCreate)
		want string
	}{
		{"missing member", func(i *discordgo.InteractionCreate) { i.Member = nil }, "unavailable"},
		{"bot user", func(i *discordgo.InteractionCreate) { i.Member.User.Bot = true }, "unavailable"},
		{"wrong guild", func(i *discordgo.InteractionCreate) { i.GuildID = "another" }, "unavailable"},
		{"missing ask", func(i *discordgo.InteractionCreate) {
			i.Data = discordgo.ApplicationCommandInteractionData{Name: "kit"}
		}, "Invalid command"},
		{"nil ask", func(i *discordgo.InteractionCreate) {
			i.Data = discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{nil}}
		}, "Invalid command"},
		{"duplicate prompt", func(i *discordgo.InteractionCreate) {
			ask := slashAsk("hello", "public")
			ask.Options = append(ask.Options, slashString("prompt", "second"))
			i.Data = discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{ask}}
		}, "Invalid command"},
		{"invalid value type", func(i *discordgo.InteractionCreate) {
			ask := slashAsk("hello", "public")
			ask.Options[0].Value = 123
			i.Data = discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{ask}}
		}, "Invalid command"},
		{"oversized prompt", func(i *discordgo.InteractionCreate) {
			i.Data = discordgo.ApplicationCommandInteractionData{Name: "kit", Options: []*discordgo.ApplicationCommandInteractionDataOption{slashAsk(strings.Repeat("x", 4001), "public")}}
		}, "Invalid command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, calls := slashTestSession(t, 200)
			bot := &DiscordBot{commands: discordCommandConfig{guild: slashTestGuild}}
			event := slashTestInteraction("kit", slashTestParent, slashTestGuild, "channel", slashAsk("hello", "public"))
			test.edit(event)
			bot.onDiscordInteraction(session, event)
			if len(*calls) != 2 || !strings.Contains(string((*calls)[0].body), `"flags":64`) ||
				!strings.Contains(string((*calls)[1].body), test.want) {
				t.Fatal("invalid request must receive a private refusal")
			}
		})
	}
	for _, event := range []*discordgo.InteractionCreate{
		nil, {}, slashTestInteraction("unknown", slashTestParent, slashTestGuild, "channel"),
		{Interaction: &discordgo.Interaction{Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{CustomID: "another-feature"}}},
	} {
		session, calls := slashTestSession(t, 200)
		(&DiscordBot{}).onDiscordInteraction(session, event)
		if len(*calls) != 0 {
			t.Fatal("unrelated interaction must be ignored")
		}
	}
}

func TestSlashStaffAuthorizationAndHelp(t *testing.T) {
	oldCamp := globalCampClient
	t.Cleanup(func() { globalCampClient = oldCamp })
	globalCampClient = &CampClient{
		allowedIDs:      map[string]bool{slashTestStaff: true},
		allowedChannels: map[string]bool{"staff-channel": true},
	}
	for _, test := range []struct {
		name, actor, guild, channel string
		campAccess, attendanceStaff bool
	}{
		{"staff approved channel", slashTestStaff, slashTestGuild, "staff-channel", true, true},
		{"staff public channel", slashTestStaff, slashTestGuild, "public", false, true},
		{"staff DM", slashTestStaff, "", "dm", true, false},
		{"nonstaff", slashTestParent, slashTestGuild, "staff-channel", false, false},
		{"wrong guild", slashTestStaff, "another-guild", "staff-channel", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, calls := slashTestSession(t, 200)
			bot := &DiscordBot{
				commands: discordCommandConfig{guild: slashTestGuild, staff: map[string]bool{slashTestStaff: true}},
				attendance: &discordAttendance{client: &attendanceClient{
					guild: slashTestGuild, staff: map[string]bool{slashTestStaff: true},
				}},
			}
			event := slashTestInteraction("camp", test.actor, test.guild, test.channel)
			if got := bot.slashCampAccess(session, event.Interaction); got != test.campAccess {
				t.Fatalf("camp access=%v, want %v", got, test.campAccess)
			}
			help := bot.slashHelp(session, event.Interaction)
			if strings.Contains(help, "`/camp ") != test.campAccess ||
				strings.Contains(help, "`/attendance staff ") != test.attendanceStaff {
				t.Fatalf("help exposed unauthorized commands: %s", help)
			}
			// A help-only camp query must never contact the registration backend.
			bot.onDiscordInteraction(session, event)
			if len(*calls) != 2 || !strings.Contains(string((*calls)[0].body), `"flags":64`) {
				t.Fatal("camp response was not private")
			}
			response := string((*calls)[1].body)
			if test.campAccess != strings.Contains(response, "Camp Power-Up Commands") {
				t.Fatalf("camp authorization response: %s", response)
			}
		})
	}
}

func TestSlashCampRegistrationBoundary(t *testing.T) {
	oldCamp := globalCampClient
	t.Cleanup(func() { globalCampClient = oldCamp })
	for _, test := range []struct {
		name, actor, channel, command string
		staff, campAllowed, role      bool
		wantData                      bool
	}{
		{"staff roster", slashTestStaff, "staff", "camp", true, true, false, true},
		{"staff thread", slashTestStaff, "thread", "camp", true, true, false, true},
		{"role-authorized staff", slashTestStaff, "staff", "camp", true, false, true, true},
		{"missing legacy camp access", slashTestStaff, "staff", "camp", true, false, false, false},
		{"nonstaff legacy access", slashTestParent, "staff", "camp", false, true, false, false},
		{"nonstaff with role", slashTestParent, "staff", "camp", false, false, true, false},
		{"staff public channel", slashTestStaff, "public", "camp", true, true, false, false},
		{"camp via public AI", slashTestStaff, "staff", "kit", true, true, false, true},
		{"denied camp via public AI", slashTestParent, "staff", "kit", false, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			exports := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				exports++
				if r.URL.Path != "/admin/export-json" {
					t.Errorf("unexpected camp endpoint: %s", r.URL.Path)
				}
				fmt.Fprint(w, `{"success":true,"registrations":[{"child_first_name":"Synthetic","child_last_name":"Camper"}]}`)
			}))
			defer server.Close()
			globalCampClient = NewCampClient(server.URL, "", "", "", "", 10)
			globalCampClient.SetAllowedChannels("staff")
			if test.campAllowed {
				globalCampClient.allowedIDs[test.actor] = true
			}
			session, calls := slashTestSession(t, 200)
			for _, channel := range []*discordgo.Channel{
				{ID: "staff", GuildID: slashTestGuild},
				{ID: "public", GuildID: slashTestGuild},
				{ID: "thread", GuildID: slashTestGuild, Type: discordgo.ChannelTypeGuildPublicThread, ParentID: "staff"},
			} {
				if err := session.State.ChannelAdd(channel); err != nil {
					t.Fatal(err)
				}
			}
			staff := map[string]bool{}
			if test.staff {
				staff[test.actor] = true
			}
			service := NewAIService(nil, nil, providerFunc{name: "must-not-run", fn: func(context.Context, string, *Session) (string, error) {
				t.Error("registration query reached AI")
				return "invalid", nil
			}})
			bot := &DiscordBot{commands: discordCommandConfig{guild: slashTestGuild, staff: staff}, aiService: service}
			option := slashString("query", "roster")
			if test.command == "kit" {
				option = slashAsk("who registered for camp?", "public")
			}
			event := slashTestInteraction(test.command, test.actor, slashTestGuild, test.channel, option)
			if test.role {
				globalCampClient.allowedRole = "Camp Staff"
				event.Member.Roles = []string{"staff-role"}
				if err := session.State.GuildAdd(&discordgo.Guild{
					ID: slashTestGuild, Roles: []*discordgo.Role{{ID: "staff-role", Name: "Camp Staff"}},
				}); err != nil {
					t.Fatal(err)
				}
				originalTransport := session.Client.Transport
				session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/roles") {
						return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
							Body: io.NopCloser(strings.NewReader(`[{"id":"staff-role","name":"Camp Staff"}]`))}, nil
					}
					return originalTransport.RoundTrip(r)
				})
			}
			bot.onDiscordInteraction(session, event)
			if len(*calls) != 2 || !strings.Contains(string((*calls)[0].body), `"flags":64`) {
				t.Fatal("camp response must stay private")
			}
			gotData := strings.Contains(string((*calls)[1].body), "Synthetic")
			if gotData != test.wantData || (exports > 0) != test.wantData {
				t.Fatalf("camp boundary failed: data=%v exports=%d response=%s", gotData, exports, (*calls)[1].body)
			}
		})
	}
}

func TestSlashAttendanceStaffDenied(t *testing.T) {
	for _, actor := range []string{"200", "300"} {
		t.Run(actor, func(t *testing.T) {
			backendCalls := 0
			attendance, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
				backendCalls++
				fmt.Fprint(w, `{"items":[]}`)
			})
			session, calls := slashTestSession(t, 200)
			bot := &DiscordBot{attendance: attendance}
			event := &discordgo.InteractionCreate{Interaction: attendanceStart(actor, "staff", date)}
			event.AppID, event.Token = slashTestApp, "synthetic"
			bot.onDiscordInteraction(session, event)
			if len(*calls) != 2 || !strings.Contains(string((*calls)[0].body), `"flags":64`) {
				t.Fatal("attendance did not route privately")
			}
			if (backendCalls > 0) != (actor == "300") {
				t.Fatal("nonstaff reached staff backend")
			}
		})
	}
}

func TestSlashLongRepliesAndDeliveryFailures(t *testing.T) {
	for _, visibility := range []string{"private", "public"} {
		t.Run(visibility, func(t *testing.T) {
			session, calls := slashTestSession(t, 200)
			answer := strings.Repeat("x😀", 1800)
			bot := &DiscordBot{aiService: NewAIService(nil, nil, providerFunc{
				name: "synthetic", fn: func(context.Context, string, *Session) (string, error) { return answer, nil },
			})}
			bot.onDiscordInteraction(session, slashTestInteraction("kit", slashTestParent, slashTestGuild, "channel", slashAsk("hello", visibility)))
			var joined strings.Builder
			for index, call := range (*calls)[1:] {
				var payload struct {
					Content         string                            `json:"content"`
					Flags           discordgo.MessageFlags            `json:"flags"`
					AllowedMentions *discordgo.MessageAllowedMentions `json:"allowed_mentions"`
				}
				if err := json.Unmarshal(call.body, &payload); err != nil {
					t.Fatal(err)
				}
				if !utf8.ValidString(payload.Content) || utf16Length(payload.Content) > 2000 ||
					payload.AllowedMentions == nil || len(payload.AllowedMentions.Parse) != 0 {
					t.Fatal("invalid chunk or mentions")
				}
				if index > 0 && ((payload.Flags&discordgo.MessageFlagsEphemeral != 0) != (visibility == "private")) {
					t.Fatal("followup changed visibility")
				}
				joined.WriteString(payload.Content)
			}
			if joined.String() != answer || len(*calls) < 3 {
				t.Fatal("long response lost content")
			}
		})
	}
	for _, status := range []int{http.StatusForbidden, http.StatusBadRequest} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			session, calls := slashTestSession(t, status)
			bot := &DiscordBot{}
			bot.onDiscordInteraction(session, slashTestInteraction("myid", slashTestParent, slashTestGuild, "channel"))
			if len(*calls) != 1 {
				t.Fatal("failed acknowledgement must not send any fallback")
			}
		})
	}
}

func TestDiscordCommandRegistration(t *testing.T) {
	for _, test := range []struct {
		name, guild, application string
		enabled                  bool
		status, wantCalls        int
	}{
		{"enabled", slashTestGuild, slashTestApp, true, 200, 1},
		{"bot ID fallback", slashTestGuild, "", true, 200, 1},
		{"disabled", slashTestGuild, slashTestApp, false, 200, 0},
		{"no guild", "", slashTestApp, true, 200, 0},
		{"invalid guild", "bad", slashTestApp, true, 200, 0},
		{"invalid application", slashTestGuild, "bad", true, 200, 0},
		{"registration failure", slashTestGuild, slashTestApp, true, 403, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, calls := slashTestSession(t, test.status)
			bot := &DiscordBot{botID: slashTestApp, commands: discordCommandConfig{
				guild: test.guild, application: test.application, register: test.enabled,
			}}
			bot.registerDiscordCommands(session)
			if len(*calls) != test.wantCalls {
				t.Fatalf("registration calls=%d, want %d", len(*calls), test.wantCalls)
			}
			if test.wantCalls == 0 {
				return
			}
			call := (*calls)[0]
			wantPath := "/api/v9/applications/" + slashTestApp + "/guilds/" + slashTestGuild + "/commands"
			if call.method != http.MethodPut || call.path != wantPath {
				t.Fatalf("registration must be a guild-scoped bulk overwrite: %+v", call)
			}
			var got []*discordgo.ApplicationCommand
			if err := json.Unmarshal(call.body, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, discordCommands()) {
				t.Fatal("registration payload differs from command suite")
			}
		})
	}
}

func TestDiscordCommandEnvironment(t *testing.T) {
	for _, test := range []struct {
		name, guild, fallback, flag, staff string
		register, staffAllowed             bool
	}{
		{name: "empty"},
		{name: "guild default", guild: slashTestGuild, register: true},
		{name: "attendance fallback", fallback: slashTestGuild, register: true},
		{name: "off", guild: slashTestGuild, flag: "false"},
		{name: "explicit on", guild: slashTestGuild, flag: "true", register: true},
		{name: "bad flag", guild: slashTestGuild, flag: "invalid"},
		{name: "staff", staff: `["` + slashTestStaff + `"]`, staffAllowed: true},
		{name: "bad staff JSON", staff: slashTestStaff},
		{name: "partial invalid staff", staff: `["` + slashTestStaff + `","bad"]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DISCORD_GUILD_ID", test.guild)
			t.Setenv("DISCORD_APPLICATION_ID", "")
			t.Setenv("DISCORD_ATTENDANCE_GUILD_ID", test.fallback)
			t.Setenv("DISCORD_REGISTER_COMMANDS", test.flag)
			t.Setenv("DISCORD_ATTENDANCE_STAFF_IDS", test.staff)
			config := discordCommandsFromEnv()
			wantGuild := test.guild
			if wantGuild == "" {
				wantGuild = test.fallback
			}
			if config.guild != wantGuild || config.register != test.register || config.staff[slashTestStaff] != test.staffAllowed {
				t.Fatalf("unexpected parsed config: %+v", config)
			}
		})
	}
}
