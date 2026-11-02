package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
)

const attendancePrefix = "/api/discord/attendance/v1"

var attendanceID = regexp.MustCompile(`^[0-9]{1,20}$`)
var attendanceHandle = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

type attendanceSession struct {
	Label string   `json:"label"`
	Dates []string `json:"dates"`
}

type attendanceClient struct {
	base, guild, parentToken, staffToken string
	staff                                map[string]bool
	sessions                             map[string]attendanceSession
	http                                 *http.Client
}

type attendanceStatus struct {
	CamperID  string  `json:"camper_id"`
	SessionID string  `json:"session_id"`
	Date      string  `json:"date"`
	State     string  `json:"state"`
	Version   int     `json:"version"`
	UpdatedAt *string `json:"updated_at"`
}

type attendancePickup struct {
	ID   string `json:"pickup_id"`
	Name string `json:"display_name"`
}

type attendanceWrite struct {
	CamperID        string `json:"camper_id"`
	SessionID       string `json:"session_id"`
	Date            string `json:"date"`
	Action          string `json:"action"`
	ExpectedVersion *int   `json:"expected_version,omitempty"`
	PickupID        string `json:"pickup_id,omitempty"`
	RosterVerified  bool   `json:"roster_verified,omitempty"`
}

type attendanceError string

func (e attendanceError) Error() string { return string(e) }

// Only known error codes cross this boundary, never server bodies or transport errors.
func attendanceMessage(err error) string {
	var code attendanceError
	if errors.As(err, &code) {
		switch code {
		case "forbidden", "unauthenticated", "not_found":
			return "Access unavailable. Ask camp leadership to verify your account, family link, or staff access privately."
		case "stale_version", "invalid_transition", "idempotency_conflict":
			return "Attendance changed or this action is no longer valid. Reopen /attendance and review fresh status before acting."
		case "invalid_request":
			return "This action is unavailable for that session/date or selection. Writes are only allowed on today's camp date in America/Chicago."
		case "unknown_outcome":
			return "The outcome is unknown. Do not assume arrival or release succeeded, or submit another confirmation blindly. Reopen /attendance to refresh status and reconcile with camp leadership."
		}
	}
	return "Private attendance is unavailable. Contact camp leadership; no public fallback is used."
}

func attendanceFromEnv() (*attendanceClient, error) {
	if os.Getenv("DISCORD_ATTENDANCE_ENABLED") != "true" {
		return nil, nil
	}
	base, err := url.Parse(os.Getenv("DISCORD_ATTENDANCE_API_BASE_URL"))
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil ||
		base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, attendanceError("invalid_config")
	}
	c := &attendanceClient{
		base:        strings.TrimRight(base.String(), "/"),
		guild:       os.Getenv("DISCORD_ATTENDANCE_GUILD_ID"),
		parentToken: os.Getenv("DISCORD_ATTENDANCE_PARENT_TOKEN"),
		staffToken:  os.Getenv("DISCORD_ATTENDANCE_STAFF_TOKEN"),
		staff:       make(map[string]bool),
		http: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
	var staff []string
	if !attendanceID.MatchString(c.guild) || len(c.parentToken) < 32 || len(c.staffToken) < 32 ||
		c.parentToken == c.staffToken || strings.ContainsAny(c.parentToken+c.staffToken, "\r\n") ||
		json.Unmarshal([]byte(os.Getenv("DISCORD_ATTENDANCE_STAFF_IDS")), &staff) != nil ||
		json.Unmarshal([]byte(os.Getenv("DISCORD_ATTENDANCE_SESSIONS")), &c.sessions) != nil ||
		len(c.sessions) == 0 || len(c.sessions) > 25 || len(staff) == 0 {
		return nil, attendanceError("invalid_config")
	}
	for _, id := range staff {
		if !attendanceID.MatchString(id) {
			return nil, attendanceError("invalid_config")
		}
		c.staff[id] = true
	}
	for slug, session := range c.sessions {
		if !attendanceHandle.MatchString(slug) || len(session.Dates) == 0 || len(session.Dates) > 25 {
			return nil, attendanceError("invalid_config")
		}
		for _, date := range session.Dates {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return nil, attendanceError("invalid_config")
			}
		}
	}
	return c, nil
}

func (c *attendanceClient) validDate(session, date string) bool {
	for _, d := range c.sessions[session].Dates {
		if d == date {
			return true
		}
	}
	return false
}

func (c *attendanceClient) currentDate(date string) bool {
	today, err := attendanceDate(time.Now())
	return err == nil && today == date
}

func attendanceDate(now time.Time) (string, error) {
	central, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return "", err
	}
	return now.In(central).Format("2006-01-02"), nil
}

func (c *attendanceClient) call(ctx context.Context, actor, guild string, staff bool, endpoint string, query url.Values, body *attendanceWrite, key string, out interface{}) error {
	if c == nil {
		return attendanceError("attendance_disabled")
	}
	if guild != c.guild || !attendanceID.MatchString(actor) || (staff && !c.staff[actor]) {
		return attendanceError("forbidden")
	}
	token := c.parentToken
	if staff {
		token = c.staffToken
	}
	method := http.MethodGet
	var data []byte
	if body != nil {
		if !c.validDate(body.SessionID, body.Date) || !c.currentDate(body.Date) || !attendanceID.MatchString(key) {
			return attendanceError("invalid_request")
		}
		method = http.MethodPost
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return attendanceError("invalid_request")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+attendancePrefix+endpoint+"?"+query.Encode(), bytes.NewReader(data))
	if err != nil {
		return attendanceError("attendance_unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Discord-Guild-ID", guild)
	req.Header.Set("X-Discord-User-ID", actor)
	if body != nil {
		// Idempotency-Key makes POST replayable to net/http; retain unknown outcomes instead.
		req.GetBody = nil
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if body != nil {
			return attendanceError("unknown_outcome")
		}
		return attendanceError("attendance_unavailable")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 262145))
	if err != nil || len(raw) > 262144 {
		if body != nil {
			return attendanceError("unknown_outcome")
		}
		return attendanceError("attendance_unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &failure) == nil {
			switch failure.Error.Code {
			case "invalid_request", "unauthenticated", "forbidden", "not_found", "invalid_transition", "stale_version", "idempotency_conflict", "attendance_disabled", "attendance_unavailable":
				return attendanceError(failure.Error.Code)
			}
		}
		if body != nil {
			return attendanceError("unknown_outcome")
		}
		return attendanceError("attendance_unavailable")
	}
	if json.Unmarshal(raw, out) != nil {
		if body != nil {
			return attendanceError("unknown_outcome")
		}
		return attendanceError("attendance_unavailable")
	}
	return nil
}

func attendanceQuery(session, date, camper string) url.Values {
	q := url.Values{"session_id": {session}}
	if date != "" {
		q.Set("date", date)
	}
	if camper != "" {
		q.Set("camper_id", camper)
	}
	return q
}

func (c *attendanceClient) status(ctx context.Context, actor, guild string, staff bool, session, date, camper string) (attendanceStatus, error) {
	endpoint := "/status"
	if staff {
		endpoint = "/staff-status"
	}
	var status attendanceStatus
	err := c.call(ctx, actor, guild, staff, endpoint, attendanceQuery(session, date, camper), nil, "", &status)
	if err == nil && !validAttendanceStatus(status, session, date, camper) {
		err = attendanceError("attendance_unavailable")
	}
	return status, err
}

func validAttendanceStatus(s attendanceStatus, session, date, camper string) bool {
	if s.SessionID != session || s.Date != date || !attendanceHandle.MatchString(s.CamperID) || (camper != "" && s.CamperID != camper) || s.Version < 0 {
		return false
	}
	switch s.State {
	case "not_arrived", "arrival_requested", "present", "pickup_requested", "released":
		return true
	}
	return false
}
