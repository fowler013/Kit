package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func attendanceFixture(t *testing.T, handler http.HandlerFunc) (*discordAttendance, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	central, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Now().In(central).Format("2006-01-02")
	client := &attendanceClient{
		base: server.URL, guild: "100", parentToken: "synthetic-parent", staffToken: "synthetic-staff",
		staff:    map[string]bool{"300": true},
		sessions: map[string]attendanceSession{"test-camp": {Dates: []string{date}}},
		http:     &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	return &discordAttendance{client: client}, date
}

func attendanceStart(actor, sub, date string) *discordgo.Interaction {
	return &discordgo.Interaction{ID: "123", GuildID: "100",
		Member: &discordgo.Member{User: &discordgo.User{ID: actor}},
		Type:   discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "attendance", Options: []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: sub, Type: discordgo.ApplicationCommandOptionSubCommand, Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "session", Value: "test-camp"}, {Name: "date", Value: date},
			}},
		}}}
}

func attendanceClick(t *testing.T, a *discordAttendance, actor, custom, id string, values ...string) (attendanceReply, error) {
	t.Helper()
	return a.component(context.Background(), &discordgo.Interaction{
		ID: id, GuildID: "100", Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: custom, Values: values},
	}, actor)
}

func attendanceSelect(t *testing.T, reply attendanceReply) discordgo.SelectMenu {
	t.Helper()
	if len(reply.components) == 0 {
		t.Fatalf("no selection: %s", reply.content)
	}
	return reply.components[0].(discordgo.ActionsRow).Components[0].(discordgo.SelectMenu)
}

func attendanceButton(t *testing.T, reply attendanceReply, label string) string {
	t.Helper()
	for _, row := range reply.components {
		for _, component := range row.(discordgo.ActionsRow).Components {
			if b, ok := component.(discordgo.Button); ok && b.Label == label {
				return b.CustomID
			}
		}
	}
	t.Fatalf("button %q absent in %s", label, reply.content)
	return ""
}

func TestAttendancePrivateParentAndStaffFlow(t *testing.T) {
	state, version, date := "not_arrived", 0, ""
	linked, pickupActive := true, true
	var writes []attendanceWrite
	var keys []string
	var endpoints []string
	a, today := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		endpoint := strings.TrimPrefix(r.URL.Path, attendancePrefix)
		endpoints = append(endpoints, endpoint)
		staff := endpoint == "/pending" || endpoint == "/staff-status" || endpoint == "/pickup-roster" || endpoint == "/confirmations"
		actor, token := "200", "synthetic-parent"
		if staff {
			actor, token = "300", "synthetic-staff"
		}
		if r.Method == http.MethodGet && r.URL.Query().Get("session_id") != "test-camp" {
			t.Error("missing exact session query")
		}
		if endpoint == "/status" || endpoint == "/staff-status" || endpoint == "/pending" {
			if r.URL.Query().Get("date") != date {
				t.Error("missing exact date query")
			}
		}
		if endpoint == "/status" || endpoint == "/staff-status" || endpoint == "/pickup-roster" {
			if r.URL.Query().Get("camper_id") != "camper-a" {
				t.Error("missing exact camper query")
			}
		}
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-Discord-User-ID") != actor || r.Header.Get("X-Discord-Guild-ID") != "100" {
			t.Error("scope or interaction-derived identity mismatch")
		}
		w.Header().Set("Content-Type", "application/json")
		if !staff && !linked {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"code":"not_found"}}`)
			return
		}
		status := func() attendanceStatus {
			return attendanceStatus{CamperID: "camper-a", SessionID: "test-camp", Date: date, State: state, Version: version}
		}
		switch endpoint {
		case "/campers":
			fmt.Fprint(w, `{"campers":[{"camper_id":"camper-a"},{"camper_id":"camper-b"}]}`)
		case "/status", "/staff-status":
			json.NewEncoder(w).Encode(status())
		case "/pending":
			json.NewEncoder(w).Encode(map[string]interface{}{"items": []attendanceStatus{status()}})
		case "/pickup-roster":
			entries := []attendancePickup{}
			if pickupActive {
				entries = append(entries, attendancePickup{ID: "pickup-a", Name: "Synthetic Authorized Adult"})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
		case "/requests", "/confirmations":
			var body attendanceWrite
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.CamperID != "camper-a" || body.SessionID != "test-camp" || body.Date != date {
				t.Error("incorrect POST shape")
			}
			writes = append(writes, body)
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			if staff {
				if body.ExpectedVersion == nil || *body.ExpectedVersion != version {
					t.Error("missing fresh expected_version")
				}
				if body.Action == "arrival" {
					state = "present"
				} else {
					if !body.RosterVerified || body.PickupID != "pickup-a" {
						t.Error("missing explicit roster attestation")
					}
					state = "released"
				}
			} else {
				if body.ExpectedVersion != nil || body.RosterVerified || body.PickupID != "" {
					t.Error("parent acquired staff fields")
				}
				if body.Action == "arrival" {
					state = "arrival_requested"
				} else {
					state = "pickup_requested"
				}
			}
			version++
			json.NewEncoder(w).Encode(status())
		default:
			t.Errorf("unexpected endpoint %s", endpoint)
		}
	})
	date = today
	ctx := context.Background()
	parent, err := a.open(ctx, attendanceStart("200", "parent", date), "200")
	if err != nil {
		t.Fatal(err)
	}
	menu := attendanceSelect(t, parent)
	if len(menu.Options) != 2 || len(writes) != 0 {
		t.Fatal("multiple campers must require selection")
	}
	parent, err = attendanceClick(t, a, "200", menu.CustomID, "401", "camper-a")
	if err != nil {
		t.Fatal(err)
	}
	arrivalKey := attendanceButton(t, parent, "Request arrival")
	parent, err = attendanceClick(t, a, "200", arrivalKey, "402")
	if err != nil || !strings.Contains(parent.content, "NOT confirmation") || state != "arrival_requested" {
		t.Fatalf("%s %v", parent.content, err)
	}
	if len(endpoints) != 3 {
		t.Fatal("parent path contacted staff endpoints")
	}
	_, err = attendanceClick(t, a, "200", arrivalKey, "402")
	if err != nil || len(writes) != 1 {
		t.Fatal("duplicate delivery wrote twice")
	}

	staff, err := a.open(ctx, attendanceStart("300", "staff", date), "300")
	if err != nil {
		t.Fatal(err)
	}
	menu = attendanceSelect(t, staff)
	staff, err = attendanceClick(t, a, "300", menu.CustomID, "403", "camper-a")
	if err != nil {
		t.Fatal(err)
	}
	staff, err = attendanceClick(t, a, "300", attendanceButton(t, staff, "I confirm physical arrival"), "404")
	if err != nil || state != "present" || !strings.Contains(staff.content, "confirmation recorded") {
		t.Fatalf("%s %v", staff.content, err)
	}

	parent, err = a.show(ctx, attendanceView{actor: "200", guild: "100", session: "test-camp", date: date, camper: "camper-a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = attendanceClick(t, a, "200", attendanceButton(t, parent, "Request pickup"), "405")
	if err != nil || state != "pickup_requested" {
		t.Fatal(err)
	}
	staff, err = a.show(ctx, attendanceView{actor: "300", guild: "100", staff: true, session: "test-camp", date: date, camper: "camper-a"})
	if err != nil {
		t.Fatal(err)
	}
	staff, err = attendanceClick(t, a, "300", attendanceButton(t, staff, "Review authorized pickup roster"), "406")
	if err != nil {
		t.Fatal(err)
	}
	menu = attendanceSelect(t, staff)
	if menu.Options[0].Label != "Synthetic Authorized Adult" {
		t.Fatal("no private pickup name")
	}
	staff, err = attendanceClick(t, a, "300", menu.CustomID, "407", "pickup-a")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(staff.content, "Manually match") || len(writes) != 3 {
		t.Fatal("selection must not release")
	}
	_, err = attendanceClick(t, a, "300", attendanceButton(t, staff, "I verified the roster and physical release"), "408")
	if err != nil || state != "released" || len(writes) != 4 {
		t.Fatal(err)
	}
	if strings.Join(keys, ",") != "402,404,405,408" {
		t.Fatalf("keys not actual interaction IDs: %v", keys)
	}

	// Link revocation is enforced again after controls have been created.
	state = "present"
	parent, err = a.show(ctx, attendanceView{actor: "200", guild: "100", session: "test-camp", date: date, camper: "camper-a"})
	if err != nil {
		t.Fatal(err)
	}
	linked = false
	_, err = attendanceClick(t, a, "200", attendanceButton(t, parent, "Request pickup"), "409")
	if err != attendanceError("not_found") || len(writes) != 4 {
		t.Fatal("revocation bypassed")
	}
}

func TestAttendanceConfirmationFreshnessAndPickupRevocation(t *testing.T) {
	for _, test := range []struct {
		name         string
		freshVersion int
		roster       string
		want         attendanceError
	}{
		{"stale", 8, `{"entries":[{"pickup_id":"pickup-a","display_name":"Synthetic"}]}`, "stale_version"},
		{"revoked-pickup", 7, `{"entries":[]}`, "not_found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			date := ""
			a, today := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					posts++
				}
				if strings.HasSuffix(r.URL.Path, "/staff-status") {
					json.NewEncoder(w).Encode(attendanceStatus{CamperID: "camper-a", SessionID: "test-camp", Date: date, State: "pickup_requested", Version: test.freshVersion})
				} else {
					fmt.Fprint(w, test.roster)
				}
			})
			date = today
			key, err := a.store(attendanceView{actor: "300", guild: "100", staff: true, session: "test-camp", date: date, camper: "camper-a", pickup: "pickup-a", version: 7, action: "confirm-release"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = attendanceClick(t, a, "300", key, "501")
			if err != test.want || posts != 0 {
				t.Fatalf("err=%v posts=%d", err, posts)
			}
		})
	}
}

func TestAttendanceTransportUnknownAndNoBlindRetry(t *testing.T) {
	for _, mode := range []string{"transport", "bad-json", "server-error", "oversize", "wrong-status"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {})
			a.client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				if mode == "transport" {
					return nil, errors.New("SENSITIVE INTERNAL DATA")
				}
				status, body := 200, "{bad"
				if mode == "server-error" {
					status, body = 500, "SENSITIVE INTERNAL DATA"
				}
				if mode == "oversize" {
					body = strings.Repeat("x", 262145)
				}
				if mode == "wrong-status" {
					body = `{"camper_id":"other","session_id":"test-camp","date":"2001-01-01","state":"released","version":8}`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			key, _ := a.store(attendanceView{actor: "200", guild: "100", session: "test-camp", date: date, camper: "camper-a", action: "request-arrival"})
			_, err := attendanceClick(t, a, "200", key, "601")
			if err != attendanceError("unknown_outcome") || calls.Load() != 1 {
				t.Fatalf("%v calls=%d", err, calls.Load())
			}
			message := attendanceMessage(err)
			if !strings.Contains(message, "unknown") || strings.Contains(message, "SENSITIVE") {
				t.Fatal(message)
			}
		})
	}
}

func TestAttendanceConfigurationAndDates(t *testing.T) {
	t.Setenv("DISCORD_ATTENDANCE_ENABLED", "")
	if c, err := attendanceFromEnv(); c != nil || err != nil {
		t.Fatal("must default disabled")
	}
	t.Setenv("DISCORD_ATTENDANCE_ENABLED", "TRUE")
	if c, err := attendanceFromEnv(); c != nil || err != nil {
		t.Fatal("only exact true enables")
	}
	t.Setenv("DISCORD_ATTENDANCE_ENABLED", "true")
	t.Setenv("DISCORD_ATTENDANCE_API_BASE_URL", "https://synthetic.example")
	t.Setenv("DISCORD_ATTENDANCE_PARENT_TOKEN", strings.Repeat("p", 32))
	t.Setenv("DISCORD_ATTENDANCE_STAFF_TOKEN", strings.Repeat("s", 32))
	t.Setenv("DISCORD_ATTENDANCE_GUILD_ID", "100")
	t.Setenv("DISCORD_ATTENDANCE_STAFF_IDS", `["300"]`)
	t.Setenv("DISCORD_ATTENDANCE_SESSIONS", `{"thanksgiving-2026":{"label":"Camp Power-Up Thanksgiving 2026 - November 23-24, 2026","dates":["2026-11-23","2026-11-24"]}}`)
	c, err := attendanceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !c.validDate("thanksgiving-2026", "2026-11-23") || c.validDate("thanksgiving-2026", "2026-11-25") {
		t.Fatal("configured dates not honored")
	}
	if c.currentDate("2000-01-01") {
		t.Fatal("historical writes allowed")
	}
	for _, variable := range []string{"DISCORD_ATTENDANCE_STAFF_TOKEN", "DISCORD_ATTENDANCE_GUILD_ID", "DISCORD_ATTENDANCE_STAFF_IDS", "DISCORD_ATTENDANCE_SESSIONS", "DISCORD_ATTENDANCE_API_BASE_URL"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv(variable, "")
			if c, err := attendanceFromEnv(); c != nil || err == nil {
				t.Fatal("invalid configuration must fail closed")
			}
		})
	}
	for _, endpoint := range []string{"http://synthetic.example", "https://user:pass@synthetic.example", "https://synthetic.example/?token=private", "https://synthetic.example/path"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Setenv("DISCORD_ATTENDANCE_API_BASE_URL", endpoint)
			if c, err := attendanceFromEnv(); c != nil || err == nil {
				t.Fatal("insecure/ambiguous origin accepted")
			}
		})
	}
	t.Setenv("DISCORD_ATTENDANCE_STAFF_TOKEN", strings.Repeat("p", 32))
	if c, err := attendanceFromEnv(); c != nil || err == nil {
		t.Fatal("scope tokens must differ")
	}
}

func TestAttendanceIdentityIsolationExpiryAndErrors(t *testing.T) {
	calls := 0
	a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	for _, test := range []struct {
		actor, guild string
		staff        bool
	}{
		{"200", "999", false}, {"200", "", false}, {"", "100", false}, {"200", "100", true},
	} {
		err := a.client.call(context.Background(), test.actor, test.guild, test.staff, "/pending", nil, nil, "", &struct{}{})
		if err != attendanceError("forbidden") {
			t.Fatal(err)
		}
	}
	if calls != 0 {
		t.Fatal("denied identity reached backend")
	}
	key, _ := a.store(attendanceView{actor: "200", guild: "100", session: "test-camp", date: date})
	if _, ok := a.take(key, "201", "100"); ok {
		t.Fatal("another user consumed private selection")
	}
	if _, ok := a.take(key, "200", "999"); ok {
		t.Fatal("another guild consumed private selection")
	}
	if _, ok := a.take(key, "200", "100"); !ok {
		t.Fatal("wrong actor consumed legitimate control")
	}
	key, _ = a.store(attendanceView{actor: "200", guild: "100"})
	a.mu.Lock()
	expired := a.views[key]
	expired.expires = time.Now().Add(-time.Minute)
	a.views[key] = expired
	a.mu.Unlock()
	if _, ok := a.take(key, "200", "100"); ok {
		t.Fatal("expired control accepted")
	}
	for _, code := range []string{"invalid_request", "unauthenticated", "forbidden", "not_found", "invalid_transition", "stale_version", "idempotency_conflict", "attendance_disabled", "attendance_unavailable"} {
		t.Run(code, func(t *testing.T) {
			a.client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"` + code + `"},"private":"never display this"}`)), Header: http.Header{}}, nil
			})
			err := a.client.call(context.Background(), "200", "100", false, "/campers", nil, nil, "", &struct{}{})
			if err != attendanceError(code) || strings.Contains(attendanceMessage(err), "never display") {
				t.Fatal(err)
			}
		})
	}
}

func TestAttendancePaginationReauthorizes(t *testing.T) {
	revoked := false
	a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if revoked {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"code":"not_found"}}`)
			return
		}
		campers := make([]map[string]string, 26)
		for i := range campers {
			campers[i] = map[string]string{"camper_id": fmt.Sprintf("camper-%02d", i)}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"campers": campers})
	})
	reply, err := a.open(context.Background(), attendanceStart("200", "parent", date), "200")
	if err != nil {
		t.Fatal(err)
	}
	if len(attendanceSelect(t, reply).Options) != 25 {
		t.Fatal("invalid first page")
	}
	key := attendanceButton(t, reply, "Next entries")
	next, err := attendanceClick(t, a, "200", key, "701")
	if err != nil || len(attendanceSelect(t, next).Options) != 1 {
		t.Fatal(err)
	}
	reply, _ = a.open(context.Background(), attendanceStart("200", "parent", date), "200")
	revoked = true
	_, err = attendanceClick(t, a, "200", attendanceButton(t, reply, "Next entries"), "702")
	if err != attendanceError("not_found") {
		t.Fatal("cached list leaked revoked access")
	}
}

func TestAttendanceOnlyEphemeralInteractionResponses(t *testing.T) {
	for _, scenario := range []string{"disabled", "wrong-guild", "missing-member", "backend-error", "parent", "ack-failure", "response-failure"} {
		t.Run(scenario, func(t *testing.T) {
			a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if scenario == "backend-error" {
					w.WriteHeader(503)
					fmt.Fprint(w, `{"error":{"code":"attendance_unavailable"}}`)
					return
				}
				fmt.Fprint(w, `{"campers":[]}`)
			})
			bot := &DiscordBot{attendance: a}
			if scenario == "disabled" {
				bot.attendance = nil
			}
			session, err := discordgo.New("Bot synthetic")
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				raw, _ := io.ReadAll(r.Body)
				if strings.Contains(r.URL.Path, "/channels/") {
					t.Fatal("public channel fallback")
				}
				if r.Method == http.MethodPost {
					var response discordgo.InteractionResponse
					if err := json.Unmarshal(raw, &response); err != nil {
						t.Fatal(err)
					}
					if response.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource || response.Data.Flags != discordgo.MessageFlagsEphemeral {
						t.Fatal("nonprivate acknowledgement")
					}
					if scenario == "ack-failure" {
						return nil, errors.New("synthetic acknowledgement failure")
					}
				} else if r.Method == http.MethodPatch {
					var response discordgo.WebhookEdit
					if err := json.Unmarshal(raw, &response); err != nil {
						t.Fatal(err)
					}
					if response.AllowedMentions == nil || len(response.AllowedMentions.Parse) != 0 {
						t.Fatal("mentions not suppressed")
					}
					if scenario == "response-failure" {
						return nil, errors.New("synthetic edit failure")
					}
				} else {
					t.Fatal("unexpected response method")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"900"}`))}, nil
			})
			interaction := attendanceStart("200", "parent", date)
			interaction.AppID, interaction.Token = "800", "synthetic"
			if scenario == "wrong-guild" {
				interaction.GuildID = "999"
			}
			if scenario == "missing-member" {
				interaction.Member = nil
			}
			bot.onAttendanceInteraction(session, &discordgo.InteractionCreate{Interaction: interaction})
			want := 2
			if scenario == "ack-failure" {
				want = 1
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
		})
	}
}

func TestAttendanceNoRedirectCredentialDisclosure(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	a, _ := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	err := a.client.call(context.Background(), "200", "100", false, "/campers", url.Values{}, nil, "", &struct{}{})
	if err == nil || targetCalls.Load() != 0 {
		t.Fatal("followed credential-bearing redirect")
	}
}

func TestAttendanceControlSingleUseConcurrent(t *testing.T) {
	a := &discordAttendance{}
	key, err := a.store(attendanceView{actor: "200", guild: "100"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var consumed atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := a.take(key, "200", "100"); ok {
				consumed.Add(1)
			}
		}()
	}
	wg.Wait()
	if consumed.Load() != 1 {
		t.Fatal("control consumed more than once")
	}
}

func TestAttendanceCentralBoundaryAndHistoricalRead(t *testing.T) {
	for _, test := range []struct{ utc, date string }{
		{"2026-11-23T05:59:59Z", "2026-11-22"},
		{"2026-11-23T06:00:00Z", "2026-11-23"},
		{"2026-11-24T05:59:59Z", "2026-11-23"},
		{"2026-11-24T06:00:00Z", "2026-11-24"},
		{"2026-11-25T06:00:00Z", "2026-11-25"},
	} {
		now, err := time.Parse(time.RFC3339, test.utc)
		if err != nil {
			t.Fatal(err)
		}
		if date, err := attendanceDate(now); err != nil || date != test.date {
			t.Fatalf("%s: %s %v", test.utc, date, err)
		}
	}
	calls := 0
	a, _ := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("historical write reached backend")
		}
		json.NewEncoder(w).Encode(attendanceStatus{CamperID: "camper-a", SessionID: "test-camp", Date: "2000-01-01", State: "not_arrived"})
	})
	a.client.sessions["test-camp"] = attendanceSession{Dates: []string{"2000-01-01"}}
	reply, err := a.show(context.Background(), attendanceView{actor: "200", guild: "100", session: "test-camp", date: "2000-01-01", camper: "camper-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.components[0].(discordgo.ActionsRow).Components) != 1 {
		t.Fatal("historical read offered write")
	}
	var result attendanceStatus
	err = a.client.call(context.Background(), "200", "100", false, "/requests", nil,
		&attendanceWrite{CamperID: "camper-a", SessionID: "test-camp", Date: "2000-01-01", Action: "arrival"}, "901", &result)
	if err != attendanceError("invalid_request") || calls != 1 {
		t.Fatal("noncurrent date write was allowed")
	}
}

func TestAttendanceStaffRosterPaginationAndBackendRevocation(t *testing.T) {
	revoked := false
	a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if revoked {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"error":{"code":"forbidden"}}`)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/pickup-roster") {
			t.Error("roster paging used incorrect endpoint")
		}
		entries := make([]attendancePickup, 26)
		for i := range entries {
			entries[i] = attendancePickup{ID: fmt.Sprintf("pickup-%02d", i), Name: fmt.Sprintf("Synthetic Adult %02d", i)}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
	})
	v := attendanceView{actor: "300", guild: "100", session: "test-camp", date: date, camper: "camper-a", staff: true, action: "pickup"}
	labels, err := a.choices(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := a.selectPage(v, labels, 0)
	if err != nil {
		t.Fatal(err)
	}
	next, err := attendanceClick(t, a, "300", attendanceButton(t, reply, "Next entries"), "902")
	if err != nil || len(attendanceSelect(t, next).Options) != 1 {
		t.Fatal(err)
	}
	reply, err = a.selectPage(v, labels, 0)
	if err != nil {
		t.Fatal(err)
	}
	revoked = true
	_, err = attendanceClick(t, a, "300", attendanceButton(t, reply, "Next entries"), "903")
	if err != attendanceError("forbidden") {
		t.Fatal("local allowlist bypassed backend staff revocation")
	}
}

func TestAttendanceMissingLinksAndUnlinkedSelection(t *testing.T) {
	calls := 0
	a, date := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"campers":[]}`)
	})
	reply, err := a.open(context.Background(), attendanceStart("200", "parent", date), "200")
	if err != nil || len(reply.components) != 0 || !strings.Contains(reply.content, "No linked campers") {
		t.Fatalf("%s %v", reply.content, err)
	}
	key, err := a.store(attendanceView{actor: "200", guild: "100", session: "test-camp", date: date, action: "camper", choices: map[string]string{"camper-a": "camper-a"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = attendanceClick(t, a, "200", key, "904", "unlinked-camper")
	if err != attendanceError("invalid_request") || calls != 1 {
		t.Fatal("tampered selection contacted backend")
	}
	interaction := attendanceStart("200", "parent", date)
	interaction.ApplicationCommandData().Options[0].Options = append(interaction.ApplicationCommandData().Options[0].Options, &discordgo.ApplicationCommandInteractionDataOption{Name: "camper", Value: "guessed"})
	if _, err := a.open(context.Background(), interaction, "200"); err != attendanceError("invalid_request") {
		t.Fatal("parent-supplied camper command option accepted")
	}
}

func TestAttendanceCommandDefinitionAndDirectStaffArrival(t *testing.T) {
	command := attendanceCommand()
	if command.Name != "attendance" || command.DMPermission == nil || *command.DMPermission ||
		command.DefaultMemberPermissions == nil || *command.DefaultMemberPermissions != int64(discordgo.PermissionUseSlashCommands) {
		t.Fatal("command definition changed privacy/permission defaults")
	}
	if len(command.Options) != 2 || len(command.Options[0].Options) != 2 || len(command.Options[1].Options) != 3 {
		t.Fatal("wrong slash options")
	}
	for _, sub := range command.Options {
		for i, option := range sub.Options {
			if option.Type != discordgo.ApplicationCommandOptionString || option.Required != (i < 2) {
				t.Fatal("wrong option schema")
			}
		}
	}
	date := ""
	a, today := attendanceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(attendanceStatus{CamperID: "camper-a", SessionID: "test-camp", Date: date, State: "not_arrived"})
	})
	date = today
	interaction := attendanceStart("300", "staff", date)
	interaction.ApplicationCommandData().Options[0].Options = append(interaction.ApplicationCommandData().Options[0].Options, &discordgo.ApplicationCommandInteractionDataOption{Name: "camper", Value: "camper-a"})
	reply, err := a.open(context.Background(), interaction, "300")
	if err != nil {
		t.Fatal(err)
	}
	attendanceButton(t, reply, "I confirm physical arrival")
}
