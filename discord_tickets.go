package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const ticketWarning = "No medical, family, or camper personal info. Not for emergencies or safety reports."
const ticketWindow = 10 * time.Minute

type ticketForm struct {
	id, kind, guild, channel string
	created                  time.Time
	used                     bool
}

type ticketStore struct {
	mu       sync.Mutex
	forms    map[string]ticketForm
	attempts map[string][]time.Time
	// Serialize read/modify/write without blocking initial modal acknowledgements.
	actions sync.Mutex
}

func ticketCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: "ticket", Description: "Staff request and feedback intake (no personal information)",
		Options: []*discordgo.ApplicationCommandOption{{
			Name: "new", Description: "Open a staff ticket form", Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandOption{{
				Name: "type", Description: "Ticket category", Type: discordgo.ApplicationCommandOptionString, Required: true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "request", Value: "request"}, {Name: "feedback", Value: "feedback"},
					{Name: "bug", Value: "bug"}, {Name: "idea", Value: "idea"},
				},
			}},
		}},
	}
}

func ticketKind(kind string) bool {
	return kind == "request" || kind == "feedback" || kind == "bug" || kind == "idea"
}

func isTicketInteraction(i *discordgo.Interaction) bool {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		return i.ApplicationCommandData().Name == "ticket"
	case discordgo.InteractionModalSubmit:
		return strings.HasPrefix(i.ModalSubmitData().CustomID, "ticket:")
	case discordgo.InteractionMessageComponent:
		return strings.HasPrefix(i.MessageComponentData().CustomID, "ticket:")
	}
	return false
}

func ticketRandomID() (string, error) {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func ticketMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
}

func ticketText(text string) string {
	text = strings.ReplaceAll(text, "@everyone", "@\u200beveryone")
	return strings.ReplaceAll(text, "@here", "@\u200bhere")
}

func ticketCommandKind(data discordgo.ApplicationCommandInteractionData) (string, bool) {
	if len(data.Options) != 1 || data.Options[0] == nil {
		return "", false
	}
	sub := data.Options[0]
	if sub.Name != "new" || sub.Type != discordgo.ApplicationCommandOptionSubCommand || len(sub.Options) != 1 || sub.Options[0] == nil {
		return "", false
	}
	option := sub.Options[0]
	kind, ok := option.Value.(string)
	return kind, ok && option.Name == "type" && option.Type == discordgo.ApplicationCommandOptionString && ticketKind(kind)
}

func ticketFields(data discordgo.ModalSubmitInteractionData) (string, string, bool) {
	values := map[string]string{}
	for _, component := range data.Components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok || row == nil || len(row.Components) != 1 {
			return "", "", false
		}
		input, ok := row.Components[0].(*discordgo.TextInput)
		if !ok || input == nil || (input.CustomID != "title" && input.CustomID != "details") {
			return "", "", false
		}
		if _, exists := values[input.CustomID]; exists {
			return "", "", false
		}
		values[input.CustomID] = strings.TrimSpace(input.Value)
	}
	title, details := values["title"], values["details"]
	valid := len(values) == 2 && title != "" && details != "" &&
		utf16Length(title) <= 100 && utf16Length(details) <= 1500
	title, details = ticketText(title), ticketText(details)
	return title, details, valid && utf16Length(title) <= 100 && utf16Length(details) <= 1500
}

func (store *ticketStore) allow(user string, now time.Time) bool {
	if store.attempts == nil {
		store.attempts = make(map[string][]time.Time)
	}
	recent := []time.Time{}
	for _, attempt := range store.attempts[user] {
		if now.Sub(attempt) < ticketWindow {
			recent = append(recent, attempt)
		}
	}
	store.attempts[user] = recent
	if len(recent) >= 5 {
		return false
	}
	store.attempts[user] = append(recent, now)
	return true
}

func (d *DiscordBot) ticketReply(s *discordgo.Session, i *discordgo.Interaction, content string, initial bool) {
	var err error
	if initial {
		err = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content, Flags: discordgo.MessageFlagsEphemeral, AllowedMentions: ticketMentions(),
			},
		})
	} else {
		_, err = s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &content, AllowedMentions: ticketMentions()})
	}
	if err != nil {
		log.Println("Kit ticket private response failed; no fallback sent")
	}
}

func (d *DiscordBot) onTicketInteraction(s *discordgo.Session, i *discordgo.Interaction) {
	if i.GuildID == "" || !d.slashStaff(i) {
		d.ticketReply(s, i, "Ticket intake is restricted to approved staff in the configured server.", true)
		return
	}
	if d.ticketsChannel == "" {
		d.ticketReply(s, i, "Ticket intake is not configured.", true)
		return
	}
	if !attendanceID.MatchString(d.ticketsChannel) {
		d.ticketReply(s, i, "Ticket destination configuration is invalid. Ask an operator to check it.", true)
		return
	}
	if i.Type == discordgo.InteractionApplicationCommand {
		kind, ok := ticketCommandKind(i.ApplicationCommandData())
		if !ok {
			d.ticketReply(s, i, "Invalid ticket options. Use /ticket new and select a type.", true)
			return
		}
		id, err := ticketRandomID()
		if err != nil {
			log.Println("Kit ticket form creation failed")
			d.ticketReply(s, i, "Ticket form is unavailable. Try again later.", true)
			return
		}
		d.tickets.mu.Lock()
		if d.tickets.forms == nil {
			d.tickets.forms = make(map[string]ticketForm)
		}
		d.tickets.forms[interactionUser(i).ID] = ticketForm{
			id: id, kind: kind, guild: i.GuildID, channel: i.ChannelID, created: time.Now(),
		}
		d.tickets.mu.Unlock()
		if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "ticket:new:" + kind + ":" + id, Title: "Staff " + kind,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
						CustomID: "title", Label: "Title", Style: discordgo.TextInputShort,
						Required: true, MaxLength: 100,
					}}},
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
						CustomID: "details", Label: "Details (no personal information)", Placeholder: ticketWarning,
						Style: discordgo.TextInputParagraph, Required: true, MaxLength: 1500,
					}}},
				},
			},
		}); err != nil {
			log.Println("Kit ticket modal acknowledgement failed; no fallback sent")
		}
		return
	}
	if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}); err != nil {
		log.Println("Kit ticket private acknowledgement failed; no fallback sent")
		return
	}
	d.tickets.actions.Lock()
	defer d.tickets.actions.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if !d.privateStaffTextChannel(ctx, s, i.GuildID, d.ticketsChannel) {
		d.ticketReply(s, i, "Tickets require a verified private staff text channel. Ask an operator to check permissions.", false)
		return
	}
	result := "Invalid ticket interaction. Reopen /ticket new."
	if i.Type == discordgo.InteractionModalSubmit {
		result = d.submitTicket(ctx, s, i)
	} else if i.Type == discordgo.InteractionMessageComponent {
		result = d.updateTicket(ctx, s, i)
	}
	d.ticketReply(s, i, result, false)
}

func ticketRequestOptions(ctx context.Context) []discordgo.RequestOption {
	return []discordgo.RequestOption{
		discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false),
	}
}

func ticketButtons(id, revision, status string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "ticket:claim:" + id + ":" + revision, Label: "Claim", Style: discordgo.PrimaryButton, Disabled: status == "Resolved"},
		discordgo.Button{CustomID: "ticket:resolve:" + id + ":" + revision, Label: "Resolve", Style: discordgo.SuccessButton, Disabled: status == "Resolved"},
		discordgo.Button{CustomID: "ticket:reopen:" + id + ":" + revision, Label: "Reopen", Style: discordgo.SecondaryButton, Disabled: status != "Resolved"},
	}}}
}

func (d *DiscordBot) submitTicket(ctx context.Context, s *discordgo.Session, i *discordgo.Interaction) string {
	data := i.ModalSubmitData()
	user := interactionUser(i).ID
	title, details, valid := ticketFields(data)
	if !valid {
		return "Invalid ticket fields. Title must be 1-100 characters and details 1-1500, including mention neutralization."
	}
	form, rejection := d.tickets.consumeForm(i, time.Now())
	if rejection != "" {
		return rejection
	}
	embed := &discordgo.MessageEmbed{
		Title: title, Description: details, Timestamp: time.Now().UTC().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{Text: "Kit ticket " + form.id},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Type", Value: form.kind}, {Name: "Submitted by", Value: "<@" + user + ">"},
			{Name: "Status", Value: "Open"},
		},
	}
	message, err := s.ChannelMessageSendComplex(d.ticketsChannel, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed}, Components: ticketButtons(form.id, form.id, "Open"),
		AllowedMentions: ticketMentions(),
	}, ticketRequestOptions(ctx)...)
	if err != nil || message == nil || message.ID == "" {
		log.Printf("Kit ticket %s delivery unconfirmed; not retried", form.id)
		return "Ticket #" + form.id + " delivery could not be confirmed. Check the configured ticket channel before submitting again."
	}
	receipt := fmt.Sprintf("Ticket #%s created: https://discord.com/channels/%s/%s/%s",
		form.id, i.GuildID, d.ticketsChannel, message.ID)
	if _, err := s.MessageThreadStart(d.ticketsChannel, message.ID, "Ticket "+form.id+" - "+form.kind, 1440, ticketRequestOptions(ctx)...); err != nil {
		log.Printf("Kit ticket %s created; discussion thread unavailable", form.id)
		return receipt + "\nThe discussion thread could not be confirmed; ask an operator to check. Do not resubmit."
	}
	log.Printf("Kit ticket %s created", form.id)
	return receipt
}

func (store *ticketStore) consumeForm(i *discordgo.Interaction, now time.Time) (ticketForm, string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	user := interactionUser(i).ID
	form, found := store.forms[user]
	if !found || form.used || now.Sub(form.created) >= ticketWindow ||
		form.guild != i.GuildID || form.channel != i.ChannelID ||
		i.ModalSubmitData().CustomID != "ticket:new:"+form.kind+":"+form.id {
		return ticketForm{}, "This form is expired, superseded, or already submitted. Check the ticket channel before opening /ticket new."
	}
	// Consume before the send: an uncertain delivery must never be blindly retried.
	form.used = true
	store.forms[user] = form
	if !store.allow(user, now) {
		return ticketForm{}, "Ticket limit reached: 5 submissions per 10 minutes. Try again later."
	}
	return form, ""
}

func ticketCurrentButton(message *discordgo.Message, customID string) bool {
	for _, component := range message.Components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok || row == nil {
			continue
		}
		for _, component := range row.Components {
			if button, ok := component.(*discordgo.Button); ok && button != nil && button.CustomID == customID && !button.Disabled {
				return true
			}
		}
	}
	return false
}

func (d *DiscordBot) updateTicket(ctx context.Context, s *discordgo.Session, i *discordgo.Interaction) string {
	parts := strings.Split(i.MessageComponentData().CustomID, ":")
	if len(parts) != 4 || parts[0] != "ticket" ||
		(parts[1] != "claim" && parts[1] != "resolve" && parts[1] != "reopen") ||
		i.ChannelID != d.ticketsChannel || i.Message == nil || i.Message.ID == "" {
		return "Invalid ticket control or destination."
	}
	message, err := s.ChannelMessage(d.ticketsChannel, i.Message.ID, ticketRequestOptions(ctx)...)
	if err != nil {
		log.Println("Kit ticket state lookup failed")
		return "Ticket state could not be verified. No update was attempted."
	}
	if message == nil || message.ID != i.Message.ID || message.ChannelID != d.ticketsChannel ||
		d.userID() == "" || message.Author == nil || message.Author.ID != d.userID() ||
		!message.Author.Bot || len(message.Embeds) != 1 || message.Embeds[0] == nil {
		return "This is not a verified Kit ticket."
	}
	embed := message.Embeds[0]
	if embed.Footer == nil || embed.Footer.Text != "Kit ticket "+parts[2] ||
		len(embed.Fields) != 3 || embed.Fields[0] == nil || embed.Fields[0].Name != "Type" ||
		!ticketKind(embed.Fields[0].Value) || embed.Fields[1] == nil || embed.Fields[1].Name != "Submitted by" ||
		embed.Fields[2] == nil || embed.Fields[2].Name != "Status" {
		return "This is not a recognized Kit ticket."
	}
	if !ticketCurrentButton(message, i.MessageComponentData().CustomID) {
		return "This control is stale or unavailable. Use the latest buttons on the ticket message."
	}
	previous := embed.Fields[2].Value
	resolved := strings.HasPrefix(previous, "Resolved \u2014 <@")
	if previous != "Open" && !strings.HasPrefix(previous, "In progress \u2014 <@") && !resolved ||
		(parts[1] == "reopen") != resolved {
		return "This transition is unavailable. Use the latest ticket controls."
	}
	revision, err := ticketRandomID()
	if err != nil {
		log.Println("Kit ticket revision creation failed")
		return "Ticket controls are unavailable. No update was attempted."
	}
	status := map[string]string{"claim": "In progress", "resolve": "Resolved", "reopen": "Open"}[parts[1]]
	embed.Fields[2].Value = status
	if parts[1] != "reopen" {
		embed.Fields[2].Value += " \u2014 <@" + interactionUser(i).ID + ">"
	}
	embed.Title, embed.Description = ticketText(embed.Title), ticketText(embed.Description)
	components := ticketButtons(parts[2], revision, status)
	empty := ""
	if _, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID: message.ID, Channel: d.ticketsChannel, Content: &empty, Embeds: &message.Embeds,
		Components: &components, AllowedMentions: ticketMentions(),
	}, ticketRequestOptions(ctx)...); err != nil {
		log.Printf("Kit ticket %s status update unconfirmed", parts[2])
		return "Status update could not be confirmed. Refresh the ticket before trying again."
	}
	if message.Flags&discordgo.MessageFlagsHasThread != 0 && (parts[1] == "resolve" || parts[1] == "reopen") {
		archived := parts[1] == "resolve"
		if _, err := s.ChannelEdit(message.ID, &discordgo.ChannelEdit{Archived: &archived}, ticketRequestOptions(ctx)...); err != nil {
			log.Printf("Kit ticket %s status updated; thread update unconfirmed", parts[2])
			return "Ticket #" + parts[2] + " status updated to " + status + ", but thread archival could not be confirmed. Ask an operator to check the thread."
		}
	}
	log.Printf("Kit ticket %s status updated", parts[2])
	return "Ticket #" + parts[2] + " status updated to " + status + "."
}
