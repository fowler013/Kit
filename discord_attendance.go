package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type attendanceView struct {
	actor, guild, session, date, camper, action, pickup string
	staff                                               bool
	version                                             int
	choices                                             map[string]string
	expires                                             time.Time
}

type discordAttendance struct {
	client *attendanceClient
	mu     sync.Mutex
	views  map[string]attendanceView
}

// This definition is not registered at startup. Operators register it only after rollout approval.
func attendanceCommand() *discordgo.ApplicationCommand {
	permission := int64(discordgo.PermissionUseSlashCommands)
	dm := false
	return &discordgo.ApplicationCommand{
		Name: "attendance", Description: "Private camp attendance requests and staff confirmations",
		DMPermission: &dm, DefaultMemberPermissions: &permission,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "parent", Description: "Select an approved linked camper privately",
				Options: attendanceCommandOptions(false)},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "staff", Description: "Review pending attendance or a known private camper handle",
				Options: attendanceCommandOptions(true)},
		},
	}
}

func attendanceCommandOptions(staff bool) []*discordgo.ApplicationCommandOption {
	opts := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "session", Description: "Configured session slug", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "date", Description: "Camp date YYYY-MM-DD (Central time)", Required: true},
	}
	if staff {
		opts = append(opts, &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionString,
			Name: "camper", Description: "Optional opaque handle from the private staff workflow"})
	}
	return opts
}

func (a *discordAttendance) store(v attendanceView) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	key := "attendance:" + hex.EncodeToString(random[:])
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.views == nil {
		a.views = make(map[string]attendanceView)
	}
	now := time.Now()
	for k, old := range a.views {
		if now.After(old.expires) {
			delete(a.views, k)
		}
	}
	if len(a.views) >= 5000 {
		return "", attendanceError("attendance_unavailable")
	}
	v.expires = now.Add(10 * time.Minute)
	a.views[key] = v
	return key, nil
}

func (a *discordAttendance) take(key, actor, guild string) (attendanceView, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.views[key]
	if !ok || v.actor != actor || v.guild != guild || time.Now().After(v.expires) {
		return attendanceView{}, false
	}
	delete(a.views, key)
	return v, true
}

type attendanceReply struct {
	content    string
	components []discordgo.MessageComponent
}

func (a *discordAttendance) button(v attendanceView, label string) (discordgo.MessageComponent, error) {
	key, err := a.store(v)
	return discordgo.Button{CustomID: key, Label: label, Style: discordgo.PrimaryButton}, err
}

// Page choices keep Discord's 25-option limit without truncating families or queues.
func (a *discordAttendance) selectPage(v attendanceView, labels map[string]string, page int) (attendanceReply, error) {
	ids := make([]string, 0, len(labels))
	for id := range labels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return attendanceReply{content: "No linked campers, pending requests, or active pickup entries are available. Contact camp leadership privately if access needs updating."}, nil
	}
	start := page * 25
	if start < 0 || start >= len(ids) {
		return attendanceReply{}, attendanceError("invalid_request")
	}
	end := start + 25
	if end > len(ids) {
		end = len(ids)
	}
	v.choices = labels
	key, err := a.store(v)
	if err != nil {
		return attendanceReply{}, err
	}
	options := make([]discordgo.SelectMenuOption, 0, end-start)
	for _, id := range ids[start:end] {
		options = append(options, discordgo.SelectMenuOption{Label: labels[id], Value: id})
	}
	reply := attendanceReply{content: "Select privately. Requests do not confirm physical arrival or authorize release.",
		components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: key, Placeholder: "Choose an entry", Options: options},
		}}}}
	if end < len(ids) {
		next := v
		next.action = fmt.Sprintf("page:%d:%s", page+1, v.action)
		button, err := a.button(next, "Next entries")
		if err != nil {
			return attendanceReply{}, err
		}
		reply.components = append(reply.components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{button}})
	}
	return reply, nil
}

func (a *discordAttendance) open(ctx context.Context, i *discordgo.Interaction, actor string) (attendanceReply, error) {
	data := i.ApplicationCommandData()
	if len(data.Options) != 1 {
		return attendanceReply{}, attendanceError("invalid_request")
	}
	sub := data.Options[0]
	if sub.Name != "parent" && sub.Name != "staff" {
		return attendanceReply{}, attendanceError("invalid_request")
	}
	v := attendanceView{actor: actor, guild: i.GuildID, staff: sub.Name == "staff"}
	for _, o := range sub.Options {
		value, ok := o.Value.(string)
		if !ok {
			return attendanceReply{}, attendanceError("invalid_request")
		}
		switch o.Name {
		case "session":
			v.session = value
		case "date":
			v.date = value
		case "camper":
			if !v.staff {
				return attendanceReply{}, attendanceError("invalid_request")
			}
			v.camper = value
		default:
			return attendanceReply{}, attendanceError("invalid_request")
		}
	}
	if !a.client.validDate(v.session, v.date) {
		return attendanceReply{}, attendanceError("invalid_request")
	}
	if v.staff && !a.client.staff[actor] {
		return attendanceReply{}, attendanceError("forbidden")
	}
	if v.camper != "" {
		return a.show(ctx, v)
	}
	v.action = "camper"
	labels, err := a.choices(ctx, v)
	if err != nil {
		return attendanceReply{}, err
	}
	return a.selectPage(v, labels, 0)
}

func (a *discordAttendance) choices(ctx context.Context, v attendanceView) (map[string]string, error) {
	labels := map[string]string{}
	if v.action == "pickup" {
		entries, err := a.roster(ctx, v)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			labels[entry.ID] = attendanceLabel(entry.Name)
		}
	} else if v.staff {
		var pending struct {
			Items []attendanceStatus `json:"items"`
		}
		err := a.client.call(ctx, v.actor, v.guild, true, "/pending", attendanceQuery(v.session, v.date, ""), nil, "", &pending)
		if err != nil {
			return nil, err
		}
		for _, s := range pending.Items {
			if !validAttendanceStatus(s, v.session, v.date, "") || (s.State != "arrival_requested" && s.State != "pickup_requested") {
				return nil, attendanceError("attendance_unavailable")
			}
			labels[s.CamperID] = attendanceLabel(s.CamperID + " - " + s.State)
		}
	} else {
		var campers struct {
			Campers []struct {
				ID string `json:"camper_id"`
			} `json:"campers"`
		}
		err := a.client.call(ctx, v.actor, v.guild, false, "/campers", attendanceQuery(v.session, "", ""), nil, "", &campers)
		if err != nil {
			return nil, err
		}
		for _, camper := range campers.Campers {
			if !attendanceHandle.MatchString(camper.ID) {
				return nil, attendanceError("attendance_unavailable")
			}
			labels[camper.ID] = attendanceLabel(camper.ID)
		}
	}
	return labels, nil
}

func attendanceLabel(text string) string {
	r := []rune(text)
	if len(r) > 100 {
		r = r[:100]
	}
	return string(r)
}

func (a *discordAttendance) show(ctx context.Context, v attendanceView) (attendanceReply, error) {
	s, err := a.client.status(ctx, v.actor, v.guild, v.staff, v.session, v.date, v.camper)
	if err != nil {
		return attendanceReply{}, err
	}
	v.version = s.Version
	v.choices = nil
	reply := attendanceReply{content: fmt.Sprintf("Camper `%s` | %s | %s\nStatus: **%s** (version %d).\nParents request only; staff must verify physical attendance. No photo ID is required.", s.CamperID, s.SessionID, s.Date, s.State, s.Version)}
	var buttons []discordgo.MessageComponent
	actions := []struct{ action, label string }{{"refresh", "Refresh status"}}
	if a.client.currentDate(v.date) {
		if v.staff {
			if s.State == "not_arrived" || s.State == "arrival_requested" {
				actions = append(actions, struct{ action, label string }{"confirm-arrival", "I confirm physical arrival"})
			}
			if s.State == "present" || s.State == "pickup_requested" {
				actions = append(actions, struct{ action, label string }{"roster", "Review authorized pickup roster"})
			}
		} else {
			if s.State == "not_arrived" {
				actions = append(actions, struct{ action, label string }{"request-arrival", "Request arrival"})
			}
			if s.State == "present" {
				actions = append(actions, struct{ action, label string }{"request-pickup", "Request pickup"})
			}
		}
	}
	for _, action := range actions {
		next := v
		next.action = action.action
		button, err := a.button(next, action.label)
		if err != nil {
			return attendanceReply{}, err
		}
		buttons = append(buttons, button)
	}
	reply.components = []discordgo.MessageComponent{discordgo.ActionsRow{Components: buttons}}
	return reply, nil
}

func (a *discordAttendance) roster(ctx context.Context, v attendanceView) ([]attendancePickup, error) {
	if !v.staff {
		return nil, attendanceError("forbidden")
	}
	var roster struct {
		Entries []attendancePickup `json:"entries"`
	}
	err := a.client.call(ctx, v.actor, v.guild, true, "/pickup-roster", attendanceQuery(v.session, "", v.camper), nil, "", &roster)
	if err == nil {
		for _, entry := range roster.Entries {
			if !attendanceHandle.MatchString(entry.ID) || strings.TrimSpace(entry.Name) == "" {
				return nil, attendanceError("attendance_unavailable")
			}
		}
	}
	return roster.Entries, err
}

func (a *discordAttendance) component(ctx context.Context, i *discordgo.Interaction, actor string) (attendanceReply, error) {
	data := i.MessageComponentData()
	v, ok := a.take(data.CustomID, actor, i.GuildID)
	if !ok {
		return attendanceReply{content: "This private control expired or was used. Reopen /attendance for current access and status."}, nil
	}
	if v.staff && !a.client.staff[actor] {
		return attendanceReply{}, attendanceError("forbidden")
	}
	if strings.HasPrefix(v.action, "page:") {
		var page int
		parts := strings.SplitN(v.action, ":", 3)
		if len(parts) != 3 {
			return attendanceReply{}, attendanceError("invalid_request")
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &page); err != nil {
			return attendanceReply{}, attendanceError("invalid_request")
		}
		v.action = parts[2]
		// Every page rechecks authoritative access and current entries.
		labels, err := a.choices(ctx, v)
		if err != nil {
			return attendanceReply{}, err
		}
		return a.selectPage(v, labels, page)
	}
	switch v.action {
	case "camper", "pickup":
		if len(data.Values) != 1 {
			return attendanceReply{}, attendanceError("invalid_request")
		}
		selected := data.Values[0]
		if _, ok := v.choices[selected]; !ok {
			return attendanceReply{}, attendanceError("invalid_request")
		}
		if v.action == "camper" {
			v.camper = selected
			return a.show(ctx, v)
		}
		v.pickup, v.action = selected, "confirm-release"
		// Show the roster name only to the backend-authorized acting staff member.
		entries, err := a.roster(ctx, v)
		if err != nil {
			return attendanceReply{}, err
		}
		for _, entry := range entries {
			if entry.ID == selected {
				button, err := a.button(v, "I verified the roster and physical release")
				name := strings.NewReplacer("*", "\\*", "_", "\\_", "`", "\\`", "~", "\\~", "\\", "\\\\").Replace(attendanceLabel(entry.Name))
				return attendanceReply{content: fmt.Sprintf("Selected authorized pickup: **%s**.\nManually match the person physically collecting the camper to the current authorized roster. Resolve uncertainty privately with the verified parent before release. A request, role, or button alone is not permission. No photo ID is required.\nConfirm only after that manual check and physical release.", name),
					components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{button}}}}, err
			}
		}
		return attendanceReply{}, attendanceError("not_found")
	case "refresh":
		return a.show(ctx, v)
	case "roster":
		s, err := a.client.status(ctx, actor, v.guild, true, v.session, v.date, v.camper)
		if err != nil {
			return attendanceReply{}, err
		}
		if s.State != "present" && s.State != "pickup_requested" {
			return attendanceReply{}, attendanceError("invalid_transition")
		}
		v.version = s.Version
		v.action = "pickup"
		labels, err := a.choices(ctx, v)
		if err != nil {
			return attendanceReply{}, err
		}
		return a.selectPage(v, labels, 0)
	case "request-arrival", "request-pickup", "confirm-arrival", "confirm-release":
		staffAction := strings.HasPrefix(v.action, "confirm-")
		if staffAction != v.staff {
			return attendanceReply{}, attendanceError("forbidden")
		}
		action := strings.TrimPrefix(strings.TrimPrefix(v.action, "request-"), "confirm-")
		body := attendanceWrite{CamperID: v.camper, SessionID: v.session, Date: v.date, Action: action}
		endpoint := "/requests"
		if v.staff {
			fresh, err := a.client.status(ctx, actor, v.guild, true, v.session, v.date, v.camper)
			if err != nil {
				return attendanceReply{}, err
			}
			if fresh.Version != v.version {
				return attendanceReply{}, attendanceError("stale_version")
			}
			body.ExpectedVersion = &fresh.Version
			endpoint = "/confirmations"
			if action == "release" {
				entries, err := a.roster(ctx, v)
				if err != nil {
					return attendanceReply{}, err
				}
				found := false
				for _, entry := range entries {
					if entry.ID == v.pickup {
						found = true
					}
				}
				if !found || v.pickup == "" {
					return attendanceReply{}, attendanceError("not_found")
				}
				body.PickupID, body.RosterVerified = v.pickup, true
			}
		}
		var result attendanceStatus
		if err := a.client.call(ctx, actor, v.guild, v.staff, endpoint, nil, &body, i.ID, &result); err != nil {
			return attendanceReply{}, err
		}
		expected := map[string]string{"arrival": "arrival_requested", "pickup": "pickup_requested"}
		if v.staff {
			expected = map[string]string{"arrival": "present", "release": "released"}
		}
		if !validAttendanceStatus(result, v.session, v.date, v.camper) || result.State != expected[action] {
			return attendanceReply{}, attendanceError("unknown_outcome")
		}
		wording := "Request recorded; this is NOT confirmation of physical arrival or release."
		if v.staff {
			wording = "Staff physical attendance confirmation recorded."
		}
		return attendanceReply{content: wording + "\nStatus: **" + result.State + "**. Reopen /attendance for fresh status before any further action."}, nil
	}
	return attendanceReply{}, attendanceError("invalid_request")
}

func (d *DiscordBot) onAttendanceInteraction(s *discordgo.Session, event *discordgo.InteractionCreate) {
	if event == nil || event.Interaction == nil {
		return
	}
	i := event.Interaction
	isCommand := i.Type == discordgo.InteractionApplicationCommand && i.ApplicationCommandData().Name == "attendance"
	isComponent := i.Type == discordgo.InteractionMessageComponent && strings.HasPrefix(i.MessageComponentData().CustomID, "attendance:")
	if !isCommand && !isComponent {
		return
	}
	// Every acknowledgement and result is ephemeral, including denied/disabled/error paths.
	if err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}); err != nil {
		log.Println("Attendance private acknowledgement failed; no fallback sent")
		return
	}
	reply := attendanceReply{}
	var err error
	if d.attendance == nil || d.attendance.client == nil {
		err = attendanceError("attendance_disabled")
	} else if i.GuildID != d.attendance.client.guild || i.Member == nil || i.Member.User == nil || i.Member.User.Bot {
		err = attendanceError("forbidden")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if isCommand {
			reply, err = d.attendance.open(ctx, i, i.Member.User.ID)
		} else {
			reply, err = d.attendance.component(ctx, i, i.Member.User.ID)
		}
	}
	if err != nil {
		reply = attendanceReply{content: attendanceMessage(err)}
	}
	components := reply.components
	if components == nil {
		components = []discordgo.MessageComponent{}
	}
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: &reply.content, Components: &components,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}); err != nil {
		log.Println("Attendance private response failed; no fallback sent")
	}
}
