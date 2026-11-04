package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

const ticketTestChannel = "123456789012345680"
const ticketTestBotID = "123456789012345681"
const ticketTestMessageID = "123456789012345682"

type ticketREST struct {
	mu      sync.Mutex
	calls   []slashRESTCall
	channel *discordgo.Channel
	message *discordgo.Message
	fail    string
}

type ticketModalAck struct {
	Type discordgo.InteractionResponseType
	Data struct {
		CustomID   string `json:"custom_id"`
		Components []*discordgo.ActionsRow
	}
}

func ticketTestSetup(t *testing.T) (*DiscordBot, *discordgo.Session, *ticketREST) {
	t.Helper()
	session, err := discordgo.New("Bot synthetic")
	if err != nil {
		t.Fatal(err)
	}
	channel := privateTestChannel()
	channel.ID = ticketTestChannel
	mock := &ticketREST{channel: channel}
	session.ShouldRetryOnRateLimit = false
	session.MaxRestRetries = 0
	session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mock.mu.Lock()
		defer mock.mu.Unlock()
		var body []byte
		if r.Body != nil {
			body, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
		}
		mock.calls = append(mock.calls, slashRESTCall{r.Method, r.URL.Path, body})
		var response interface{} = map[string]string{"id": "response"}
		status, operation := 200, ""
		switch {
		case strings.HasSuffix(r.URL.Path, "/callback"):
			operation = "ack"
		case strings.Contains(r.URL.Path, "/webhooks/"):
			operation = "receipt"
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages/"):
			operation, response = "read", mock.message
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/channels/"):
			operation, response = "channel", mock.channel
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/roles"):
			operation, response = "roles", []*discordgo.Role{{ID: "staff-role", Name: "Camp Staff"}}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			operation = "send"
			if mock.fail != operation {
				message := &discordgo.Message{}
				if err := json.Unmarshal(body, message); err != nil {
					t.Fatal(err)
				}
				message.ID, message.ChannelID = ticketTestMessageID, ticketTestChannel
				message.Author = &discordgo.User{ID: ticketTestBotID, Bot: true}
				mock.message, response = message, message
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/threads"):
			operation = "thread"
			if mock.fail != operation {
				mock.message.Flags |= discordgo.MessageFlagsHasThread
			}
			response = &discordgo.Channel{ID: ticketTestMessageID}
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/messages/"):
			operation = "edit"
			if mock.fail != operation {
				var edited discordgo.Message
				if err := json.Unmarshal(body, &edited); err != nil {
					t.Fatal(err)
				}
				mock.message.Embeds, mock.message.Components = edited.Embeds, edited.Components
				mock.message.Content = edited.Content
				response = mock.message
			}
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/"+ticketTestMessageID):
			operation = "archive"
			response = &discordgo.Channel{ID: ticketTestMessageID}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if mock.fail == operation {
			status, response = 403, map[string]string{"message": "synthetic failure"}
		}
		// discordgo only unmarshals Message.Components; its outbound JSON omits them.
		if message, ok := response.(*discordgo.Message); ok && message != nil {
			response = struct {
				*discordgo.Message
				Components []discordgo.MessageComponent `json:"components"`
			}{message, message.Components}
		}
		payload, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: status,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   io.NopCloser(strings.NewReader(string(payload)))}, nil
	})
	bot := &DiscordBot{
		botID: ticketTestBotID, ticketsChannel: ticketTestChannel,
		commands: discordCommandConfig{guild: slashTestGuild, staff: map[string]bool{slashTestStaff: true}},
	}
	return bot, session, mock
}

func ticketTestCommand() *discordgo.InteractionCreate {
	return slashTestInteraction("ticket", slashTestStaff, slashTestGuild, "entry",
		&discordgo.ApplicationCommandInteractionDataOption{
			Name: "new", Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{slashString("type", "feedback")},
		})
}

func ticketTestOpen(t *testing.T, bot *DiscordBot, s *discordgo.Session, mock *ticketREST) *discordgo.InteractionCreate {
	t.Helper()
	bot.onDiscordInteraction(s, ticketTestCommand())
	var response ticketModalAck
	if err := json.Unmarshal(mock.calls[len(mock.calls)-1].body, &response); err != nil {
		t.Fatal(err)
	}
	if response.Type != discordgo.InteractionResponseModal {
		t.Fatalf("expected modal: %s", mock.calls[len(mock.calls)-1].body)
	}
	event := slashTestInteraction("ticket", slashTestStaff, slashTestGuild, "entry")
	event.Type = discordgo.InteractionModalSubmit
	event.Data = discordgo.ModalSubmitInteractionData{
		CustomID: response.Data.CustomID,
		Components: []discordgo.MessageComponent{
			&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "title", Value: "Equipment @everyone"}}},
			&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "details", Value: "Ideas @here <@123> <@&456>"}}},
		},
	}
	return event
}

func ticketTestButton(t *testing.T, mock *ticketREST, action string) *discordgo.InteractionCreate {
	t.Helper()
	event := slashTestInteraction("ticket", slashTestStaff, slashTestGuild, ticketTestChannel)
	event.Type, event.Message = discordgo.InteractionMessageComponent, &discordgo.Message{ID: ticketTestMessageID}
	for _, component := range mock.message.Components {
		row := component.(*discordgo.ActionsRow)
		for _, component := range row.Components {
			button := component.(*discordgo.Button)
			if strings.HasPrefix(button.CustomID, "ticket:"+action+":") {
				event.Data = discordgo.MessageComponentInteractionData{CustomID: button.CustomID, ComponentType: discordgo.ButtonComponent}
				return event
			}
		}
	}
	t.Fatal("button not found")
	return nil
}

func ticketCallCount(mock *ticketREST, method, suffix string) int {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	count := 0
	for _, call := range mock.calls {
		if call.method == method && strings.HasSuffix(call.path, suffix) {
			count++
		}
	}
	return count
}

func ticketLastReply(t *testing.T, mock *ticketREST) string {
	t.Helper()
	var body struct {
		Content string `json:"content"`
		Data    struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mock.calls[len(mock.calls)-1].body, &body); err != nil {
		t.Fatal(err)
	}
	return body.Content + body.Data.Content
}

func TestTicketSchemaAndInitialModal(t *testing.T) {
	var command *discordgo.ApplicationCommand
	for _, candidate := range discordCommands() {
		if candidate.Name == "ticket" {
			command = candidate
		}
	}
	if command == nil || len(command.Options) != 1 || command.Options[0].Name != "new" ||
		command.Options[0].Type != discordgo.ApplicationCommandOptionSubCommand || len(command.Options[0].Options) != 1 {
		t.Fatal("invalid ticket subcommand")
	}
	option := command.Options[0].Options[0]
	if option.Name != "type" || !option.Required || option.Type != discordgo.ApplicationCommandOptionString || len(option.Choices) != 4 {
		t.Fatal("invalid category schema")
	}
	for index, kind := range []string{"request", "feedback", "bug", "idea"} {
		if option.Choices[index].Name != kind || option.Choices[index].Value != kind {
			t.Fatal("unexpected ticket choices")
		}
	}
	bot, s, mock := ticketTestSetup(t)
	ticketTestOpen(t, bot, s, mock)
	if len(mock.calls) != 1 || !strings.HasSuffix(mock.calls[0].path, "/callback") {
		t.Fatal("form performed work before its initial response")
	}
	var response ticketModalAck
	if err := json.Unmarshal(mock.calls[0].body, &response); err != nil {
		t.Fatal(err)
	}
	if response.Type != discordgo.InteractionResponseModal || len(response.Data.Components) != 2 {
		t.Fatal("modal was deferred or malformed")
	}
	for index, name := range []string{"title", "details"} {
		input := response.Data.Components[index].Components[0].(*discordgo.TextInput)
		limit := 100
		if index == 1 {
			limit = 1500
			if input.Placeholder != ticketWarning || input.Style != discordgo.TextInputParagraph {
				t.Fatal("safety warning missing")
			}
		}
		if input.CustomID != name || !input.Required || input.MaxLength != limit {
			t.Fatal("incorrect modal field bounds")
		}
	}
	staff := bot.slashHelp(s, ticketTestCommand().Interaction)
	parent := bot.slashHelp(s, slashTestInteraction("help", slashTestParent, slashTestGuild, "entry").Interaction)
	if !strings.Contains(staff, "/ticket") || strings.Contains(parent, "/ticket") {
		t.Fatal("ticket help is not staff-only")
	}
}

func TestTicketAuthorizationEverySurface(t *testing.T) {
	for _, phase := range []string{"command", "modal", "button"} {
		for _, deny := range []string{"nonstaff", "wrong guild", "DM", "bot", "missing member", "revoked"} {
			t.Run(phase+"/"+deny, func(t *testing.T) {
				bot, s, mock := ticketTestSetup(t)
				event := ticketTestCommand()
				if phase != "command" {
					event = ticketTestOpen(t, bot, s, mock)
				}
				if phase == "button" {
					bot.onDiscordInteraction(s, event)
					event = ticketTestButton(t, mock, "claim")
				}
				before := len(mock.calls)
				switch deny {
				case "nonstaff":
					event.Member.User.ID = slashTestParent
				case "wrong guild":
					event.GuildID = "other"
				case "DM":
					event.GuildID, event.User = "", &discordgo.User{ID: slashTestStaff}
				case "bot":
					event.Member.User.Bot = true
				case "missing member":
					event.Member = nil
				case "revoked":
					delete(bot.commands.staff, slashTestStaff)
				}
				bot.onDiscordInteraction(s, event)
				if len(mock.calls) != before+1 || !strings.Contains(ticketLastReply(t, mock), "restricted") {
					t.Fatal("unauthorized interaction performed work")
				}
				var response discordgo.InteractionResponse
				if err := json.Unmarshal(mock.calls[before].body, &response); err != nil {
					t.Fatal(err)
				}
				if response.Data.Flags != discordgo.MessageFlagsEphemeral {
					t.Fatal("denial not private")
				}
			})
		}
	}
}

func TestTicketDisabledAndInvalidConfiguration(t *testing.T) {
	for _, config := range []string{"", "not-an-id"} {
		for _, phase := range []string{"command", "modal", "button"} {
			t.Run(config+"/"+phase, func(t *testing.T) {
				bot, s, mock := ticketTestSetup(t)
				event := ticketTestCommand()
				if phase != "command" {
					event = ticketTestOpen(t, bot, s, mock)
				}
				if phase == "button" {
					bot.onDiscordInteraction(s, event)
					event = ticketTestButton(t, mock, "claim")
				}
				before := len(mock.calls)
				bot.ticketsChannel = config
				bot.onDiscordInteraction(s, event)
				if len(mock.calls) != before+1 || !strings.Contains(ticketLastReply(t, mock), "configur") {
					t.Fatal("disabled feature performed work")
				}
			})
		}
	}
}

func TestTicketDestinationPrivacy(t *testing.T) {
	for _, phase := range []string{"modal", "button"} {
		for _, reject := range []string{"public", "allow everyone", "member", "role", "nil overwrite", "wrong guild", "wrong channel", "forum", "thread", "lookup error", "role lookup error"} {
			t.Run(phase+"/"+reject, func(t *testing.T) {
				bot, s, mock := ticketTestSetup(t)
				event := ticketTestOpen(t, bot, s, mock)
				if phase == "button" {
					bot.onDiscordInteraction(s, event)
					event = ticketTestButton(t, mock, "claim")
				}
				switch reject {
				case "public":
					mock.channel.PermissionOverwrites = nil
				case "allow everyone":
					mock.channel.PermissionOverwrites[0].Allow = discordgo.PermissionViewChannel
				case "member":
					mock.channel.PermissionOverwrites[1].ID = slashTestParent
				case "role", "role lookup error":
					mock.channel.PermissionOverwrites[1].ID = "unapproved"
					mock.channel.PermissionOverwrites[1].Type = discordgo.PermissionOverwriteTypeRole
					bot.staffRole = "Camp Staff"
					if reject == "role lookup error" {
						mock.fail = "roles"
					}
				case "nil overwrite":
					mock.channel.PermissionOverwrites = append(mock.channel.PermissionOverwrites, nil)
				case "wrong guild":
					mock.channel.GuildID = "other"
				case "wrong channel":
					mock.channel.ID = "other"
				case "forum":
					mock.channel.Type = discordgo.ChannelTypeGuildForum
				case "thread":
					mock.channel.Type = discordgo.ChannelTypeGuildPublicThread
				case "lookup error":
					mock.fail = "channel"
				}
				before := len(mock.calls)
				bot.onDiscordInteraction(s, event)
				if !strings.Contains(ticketLastReply(t, mock), "verified private") {
					t.Fatal("unsafe channel not rejected")
				}
				for _, call := range mock.calls[before:] {
					if call.method != http.MethodGet && !strings.Contains(call.path, "/interactions/") && !strings.Contains(call.path, "/webhooks/") {
						t.Fatal("privacy failure mutated a destination")
					}
				}
			})
		}
	}
}

func TestTicketWorkflowRestartStaleAndMentions(t *testing.T) {
	bot, s, mock := ticketTestSetup(t)
	form := ticketTestOpen(t, bot, s, mock)
	bot.onDiscordInteraction(s, form)
	if mock.message == nil || mock.message.Embeds[0].Fields[2].Value != "Open" {
		t.Fatal("ticket not created")
	}
	embed := mock.message.Embeds[0]
	if len(strings.TrimPrefix(embed.Footer.Text, "Kit ticket ")) != 12 || embed.Fields[0].Value != "feedback" ||
		embed.Fields[1].Value != "<@"+slashTestStaff+">" || embed.Timestamp == "" ||
		strings.Contains(embed.Title+embed.Description, "@everyone") || strings.Contains(embed.Description, "@here") {
		t.Fatal("ticket fields or neutralization incorrect")
	}
	if _, err := time.Parse(time.RFC3339, embed.Timestamp); err != nil {
		t.Fatal("invalid creation time")
	}
	if !strings.Contains(ticketLastReply(t, mock), "https://discord.com/channels/"+slashTestGuild+"/"+ticketTestChannel+"/"+ticketTestMessageID) {
		t.Fatal("missing jump-link receipt")
	}
	bot.onDiscordInteraction(s, form)
	if ticketCallCount(mock, http.MethodPost, "/messages") != 1 || !strings.Contains(ticketLastReply(t, mock), "already submitted") {
		t.Fatal("duplicate form reposted")
	}
	stale := ticketTestButton(t, mock, "claim")
	// A new bot has no form, rate-limit, or ticket state; the message is sufficient.
	restarted := &DiscordBot{botID: ticketTestBotID, ticketsChannel: ticketTestChannel, commands: bot.commands}
	for index, action := range []string{"claim", "resolve", "reopen"} {
		event := ticketTestButton(t, mock, action)
		if index == 0 {
			// Event embeds are deliberately absent; never reconstruct from a stale event.
			event.Message.Embeds = nil
		}
		restarted.onDiscordInteraction(s, event)
		want := []string{"In progress \u2014 <@" + slashTestStaff + ">", "Resolved \u2014 <@" + slashTestStaff + ">", "Open"}[index]
		if got := mock.message.Embeds[0].Fields[2].Value; got != want {
			t.Fatalf("%s got %q want %q; reply %s", action, got, want, ticketLastReply(t, mock))
		}
	}
	before := ticketCallCount(mock, http.MethodPatch, "/messages/"+ticketTestMessageID)
	restarted.onDiscordInteraction(s, stale)
	if ticketCallCount(mock, http.MethodPatch, "/messages/"+ticketTestMessageID) != before || !strings.Contains(ticketLastReply(t, mock), "stale") {
		t.Fatal("stale control changed ticket")
	}
	if ticketCallCount(mock, http.MethodPatch, "/channels/"+ticketTestMessageID) != 2 {
		t.Fatal("resolve/reopen did not archive/unarchive")
	}
	archives := []bool{}
	for _, call := range mock.calls {
		if call.method == http.MethodPatch && strings.HasSuffix(call.path, "/channels/"+ticketTestMessageID) {
			var body struct{ Archived bool }
			if err := json.Unmarshal(call.body, &body); err != nil {
				t.Fatal(err)
			}
			archives = append(archives, body.Archived)
		}
		if call.method == http.MethodGet || strings.HasSuffix(call.path, "/threads") ||
			strings.HasSuffix(call.path, "/channels/"+ticketTestMessageID) {
			continue
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(call.body, &body); err != nil {
			t.Fatal(err)
		}
		if data, ok := body["data"]; ok {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(data, &nested); err != nil {
				t.Fatal(err)
			}
			var kind int
			if err := json.Unmarshal(body["type"], &kind); err != nil {
				t.Fatal(err)
			}
			if kind == int(discordgo.InteractionResponseModal) || kind == int(discordgo.InteractionResponseDeferredChannelMessageWithSource) {
				continue
			}
			body = nested
		}
		var mentions discordgo.MessageAllowedMentions
		if err := json.Unmarshal(body["allowed_mentions"], &mentions); err != nil ||
			mentions.Parse == nil || len(mentions.Parse)+len(mentions.Users)+len(mentions.Roles) != 0 || mentions.RepliedUser {
			t.Fatal("mentions not explicitly suppressed")
		}
	}
	if !reflect.DeepEqual(archives, []bool{true, false}) {
		t.Fatal("wrong thread lifecycle")
	}
}

func TestTicketRateLimit(t *testing.T) {
	store := ticketStore{}
	now := time.Now()
	for index := 0; index < 5; index++ {
		if !store.allow(slashTestStaff, now.Add(time.Duration(index)*time.Second)) {
			t.Fatal("early rate limit")
		}
	}
	if store.allow(slashTestStaff, now.Add(time.Minute)) || !store.allow(slashTestParent, now.Add(time.Minute)) ||
		!store.allow(slashTestStaff, now.Add(ticketWindow)) {
		t.Fatal("incorrect user/window limit")
	}
	bot, s, mock := ticketTestSetup(t)
	for index := 0; index < 6; index++ {
		bot.onDiscordInteraction(s, ticketTestOpen(t, bot, s, mock))
	}
	if ticketCallCount(mock, http.MethodPost, "/messages") != 5 || !strings.Contains(ticketLastReply(t, mock), "limit reached") {
		t.Fatal("handler did not enforce rate limit")
	}
}

func TestTicketMalformedAndExpiredInput(t *testing.T) {
	for _, invalid := range []string{"blank title", "long title", "long details", "mention expansion", "duplicate", "unknown", "bad row", "missing", "expired", "superseded", "restart", "other channel", "bad ID"} {
		t.Run(invalid, func(t *testing.T) {
			bot, s, mock := ticketTestSetup(t)
			event := ticketTestOpen(t, bot, s, mock)
			data := event.ModalSubmitData()
			input := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
			switch invalid {
			case "blank title":
				input.Value = " "
			case "long title":
				input.Value = strings.Repeat("x", 101)
			case "long details":
				data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value = strings.Repeat("x", 1501)
			case "mention expansion":
				input.Value = strings.Repeat("x", 91) + "@everyone"
			case "duplicate":
				data.Components = append(data.Components, data.Components[0])
			case "unknown":
				input.CustomID = "unrecognized"
			case "bad row":
				data.Components[0] = &discordgo.Button{CustomID: "title"}
			case "missing":
				data.Components = data.Components[:1]
			case "expired":
				form := bot.tickets.forms[slashTestStaff]
				form.created = time.Now().Add(-ticketWindow)
				bot.tickets.forms[slashTestStaff] = form
			case "superseded":
				ticketTestOpen(t, bot, s, mock)
			case "restart":
				bot.tickets = ticketStore{}
			case "other channel":
				event.ChannelID = "other"
			case "bad ID":
				data.CustomID = "ticket:new:bad:id"
			}
			event.Data = data
			bot.onDiscordInteraction(s, event)
			if mock.message != nil || ticketLastReply(t, mock) == "" {
				t.Fatal("invalid form sent ticket or lacked explicit rejection")
			}
		})
	}
	for _, invalid := range []string{"missing", "kind", "subcommand", "duplicate", "value"} {
		t.Run("command/"+invalid, func(t *testing.T) {
			bot, s, mock := ticketTestSetup(t)
			event := ticketTestCommand()
			data := event.ApplicationCommandData()
			switch invalid {
			case "missing":
				data.Options = nil
			case "kind":
				data.Options[0].Options[0].Value = "private"
			case "subcommand":
				data.Options[0].Name = "delete"
			case "duplicate":
				data.Options = append(data.Options, data.Options[0])
			case "value":
				data.Options[0].Options[0].Value = 1
			}
			event.Data = data
			bot.onDiscordInteraction(s, event)
			if len(mock.calls) != 1 || !strings.Contains(ticketLastReply(t, mock), "Invalid ticket") {
				t.Fatal("malformed command opened form")
			}
		})
	}
	// UTF-16 limits, not bytes or runes.
	data := discordgo.ModalSubmitInteractionData{Components: []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "title", Value: strings.Repeat("\U0001f600", 50)}}},
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&discordgo.TextInput{CustomID: "details", Value: strings.Repeat("x", 1500)}}},
	}}
	if _, _, valid := ticketFields(data); !valid {
		t.Fatal("valid maximum lengths rejected")
	}
	data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value += "x"
	if _, _, valid := ticketFields(data); valid {
		t.Fatal("overlong UTF-16 title accepted")
	}
}

func TestTicketMessageOriginAndTransitions(t *testing.T) {
	for _, invalid := range []string{"other channel", "author", "not bot", "footer", "fields", "status", "disabled", "bad control", "message ID", "missing message", "channel changed"} {
		t.Run(invalid, func(t *testing.T) {
			bot, s, mock := ticketTestSetup(t)
			bot.onDiscordInteraction(s, ticketTestOpen(t, bot, s, mock))
			event := ticketTestButton(t, mock, "claim")
			switch invalid {
			case "other channel":
				event.ChannelID = "other"
			case "author":
				mock.message.Author.ID = slashTestStaff
			case "not bot":
				mock.message.Author.Bot = false
			case "footer":
				mock.message.Embeds[0].Footer.Text = "not a ticket"
			case "fields":
				mock.message.Embeds[0].Fields = nil
			case "status":
				mock.message.Embeds[0].Fields[2].Value = "Unknown"
			case "disabled":
				event = ticketTestButton(t, mock, "reopen")
			case "bad control":
				event.Data = discordgo.MessageComponentInteractionData{CustomID: "ticket:delete:id:rev"}
			case "message ID":
				mock.message.ID = "other"
			case "missing message":
				event.Message = nil
			case "channel changed":
				bot.ticketsChannel = "123456789012345699"
			}
			bot.onDiscordInteraction(s, event)
			if ticketCallCount(mock, http.MethodPatch, "/messages/"+ticketTestMessageID) != 0 {
				t.Fatal("unverified ticket was mutated")
			}
		})
	}
}

func TestTicketAPIFailures(t *testing.T) {
	for _, failure := range []string{"ack", "send", "thread", "read", "edit", "archive", "receipt"} {
		t.Run(failure, func(t *testing.T) {
			bot, s, mock := ticketTestSetup(t)
			event := ticketTestOpen(t, bot, s, mock)
			if failure == "read" || failure == "edit" || failure == "archive" {
				bot.onDiscordInteraction(s, event)
				event = ticketTestButton(t, mock, "resolve")
			}
			mock.fail = failure
			before := len(mock.calls)
			bot.onDiscordInteraction(s, event)
			switch failure {
			case "ack":
				if len(mock.calls) != before+1 || mock.message != nil {
					t.Fatal("failed acknowledgement continued")
				}
			case "send":
				if mock.message != nil || !strings.Contains(ticketLastReply(t, mock), "could not be confirmed") {
					t.Fatal("send failure claimed success")
				}
				bot.onDiscordInteraction(s, event)
				if ticketCallCount(mock, http.MethodPost, "/messages") != 1 {
					t.Fatal("unknown delivery blindly retried")
				}
			case "thread":
				if mock.message == nil || !strings.Contains(ticketLastReply(t, mock), "Do not resubmit") {
					t.Fatal("thread failure lost ticket or concealed partial success")
				}
			case "read", "edit":
				if mock.message.Embeds[0].Fields[2].Value != "Open" || !strings.Contains(ticketLastReply(t, mock), "could not be") {
					t.Fatal("failed update claimed success")
				}
			case "archive":
				if !strings.HasPrefix(mock.message.Embeds[0].Fields[2].Value, "Resolved") || !strings.Contains(ticketLastReply(t, mock), "but thread") {
					t.Fatal("archive failure concealed partial success")
				}
			case "receipt":
				if mock.message == nil || ticketCallCount(mock, http.MethodPost, "/messages") != 1 {
					t.Fatal("receipt failure lost or duplicated ticket")
				}
			}
		})
	}
}

func TestTicketConcurrentControls(t *testing.T) {
	bot, s, mock := ticketTestSetup(t)
	bot.onDiscordInteraction(s, ticketTestOpen(t, bot, s, mock))
	one, two := ticketTestButton(t, mock, "claim"), ticketTestButton(t, mock, "resolve")
	var done sync.WaitGroup
	done.Add(2)
	for _, event := range []*discordgo.InteractionCreate{one, two} {
		go func(event *discordgo.InteractionCreate) {
			defer done.Done()
			bot.onDiscordInteraction(s, event)
		}(event)
	}
	done.Wait()
	if count := ticketCallCount(mock, http.MethodPatch, "/messages/"+ticketTestMessageID); count != 1 {
		t.Fatalf("concurrent stale controls produced %d edits", count)
	}
}

func TestTicketConcurrentSubmitAndNonblockingModal(t *testing.T) {
	bot, s, mock := ticketTestSetup(t)
	event := ticketTestOpen(t, bot, s, mock)
	var done sync.WaitGroup
	done.Add(2)
	for index := 0; index < 2; index++ {
		go func() {
			defer done.Done()
			bot.onDiscordInteraction(s, event)
		}()
	}
	done.Wait()
	if ticketCallCount(mock, http.MethodPost, "/messages") != 1 {
		t.Fatal("duplicate concurrent submission posted more than once")
	}
	bot.tickets.actions.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			bot.tickets.actions.Unlock()
		}
	}()
	opened := make(chan struct{})
	go func() {
		bot.onDiscordInteraction(s, ticketTestCommand())
		close(opened)
	}()
	select {
	case <-opened:
	case <-time.After(time.Second):
		bot.tickets.actions.Unlock()
		unlocked = true
		<-opened
		t.Fatal("modal initial acknowledgement blocked behind network operations")
	}
}

func TestTicketPrivacyApprovedRoleAndNoThread(t *testing.T) {
	bot, s, mock := ticketTestSetup(t)
	bot.staffRole = "Camp Staff"
	mock.channel.PermissionOverwrites[1].ID = "staff-role"
	mock.channel.PermissionOverwrites[1].Type = discordgo.PermissionOverwriteTypeRole
	mock.fail = "thread"
	bot.onDiscordInteraction(s, ticketTestOpen(t, bot, s, mock))
	if mock.message == nil {
		t.Fatal("approved staff role did not pass shared privacy check")
	}
	mock.fail = ""
	bot.onDiscordInteraction(s, ticketTestButton(t, mock, "resolve"))
	if !strings.HasPrefix(mock.message.Embeds[0].Fields[2].Value, "Resolved") ||
		ticketCallCount(mock, http.MethodPatch, "/channels/"+ticketTestMessageID) != 0 {
		t.Fatal("ticket without thread could not be resolved independently")
	}
}

func TestTicketAttendanceRoutingUnchanged(t *testing.T) {
	for _, kind := range []discordgo.InteractionType{discordgo.InteractionApplicationCommand, discordgo.InteractionMessageComponent, discordgo.InteractionModalSubmit} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			bot, s, mock := ticketTestSetup(t)
			event := slashTestInteraction("attendance", slashTestParent, slashTestGuild, "entry")
			event.Type = kind
			switch kind {
			case discordgo.InteractionMessageComponent:
				event.Data = discordgo.MessageComponentInteractionData{CustomID: "attendance:existing"}
			case discordgo.InteractionModalSubmit:
				event.Data = discordgo.ModalSubmitInteractionData{CustomID: "attendance:existing"}
			}
			bot.onDiscordInteraction(s, event)
			if kind == discordgo.InteractionModalSubmit {
				if len(mock.calls) != 0 {
					t.Fatal("attendance modals were previously ignored and must remain ignored")
				}
				return
			}
			if len(mock.calls) != 2 || !strings.Contains(ticketLastReply(t, mock), "attendance is unavailable") {
				t.Fatalf("attendance routing changed: %+v", mock.calls)
			}
		})
	}
	bot, s, mock := ticketTestSetup(t)
	event := ticketTestOpen(t, bot, s, mock)
	before := len(mock.calls)
	bot.onAttendanceInteraction(s, event)
	event.Type = discordgo.InteractionMessageComponent
	event.Data = discordgo.MessageComponentInteractionData{CustomID: "ticket:claim:id:rev"}
	bot.onAttendanceInteraction(s, event)
	if len(mock.calls) != before {
		t.Fatal("attendance intercepted ticket modal or button")
	}
}
