package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// CampClient fetches registration data from a Camp Power-Up deployment.
// Answers are composed directly from the data - registration PII is never
// sent to any AI provider.
type CampClient struct {
	baseURL     string
	username    string
	password    string
	allowedIDs  map[string]bool
	allowedRole string
	capacity    int
	httpClient  *http.Client
}

// NewCampClient creates a client for the Camp Power-Up registration API.
// allowedDiscordIDs is a comma-separated list of Discord user IDs permitted
// to query registration data. allowedRole is an optional Discord role name
// that also grants access. capacity is the max number of campers (0 = unknown).
func NewCampClient(baseURL, username, password, allowedDiscordIDs, allowedRole string, capacity int) *CampClient {
	if baseURL == "" {
		return nil
	}

	allowed := make(map[string]bool)
	for _, id := range strings.Split(allowedDiscordIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = true
		}
	}

	jar, _ := cookiejar.New(nil)
	return &CampClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		username:    username,
		password:    password,
		allowedIDs:  allowed,
		allowedRole: strings.TrimSpace(allowedRole),
		capacity:    capacity,
		httpClient:  &http.Client{Timeout: 30 * time.Second, Jar: jar},
	}
}

// AllowedRole returns the Discord role name that grants camp data access.
func (c *CampClient) AllowedRole() string {
	if c == nil {
		return ""
	}
	return c.allowedRole
}

// BaseURL returns the configured Camp Power-Up base URL.
func (c *CampClient) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

// RegistrationCount returns the current number of registrations.
func (c *CampClient) RegistrationCount() (int, error) {
	regs, err := c.fetchRegistrations()
	if err != nil {
		return 0, err
	}
	return len(regs), nil
}

type campExport struct {
	Success       *bool                    `json:"success"`
	Count         int                      `json:"count"`
	Registrations []map[string]interface{} `json:"registrations"`
}

func (c *CampClient) login() error {
	if c.username == "" || c.password == "" {
		return fmt.Errorf("camp admin credentials not configured")
	}
	form := url.Values{"username": {c.username}, "password": {c.password}}
	resp, err := c.httpClient.PostForm(c.baseURL+"/admin/login", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// fetchRegistrations retrieves current registrations, logging in first if needed.
func (c *CampClient) fetchRegistrations() ([]map[string]interface{}, error) {
	fetch := func() (*campExport, error) {
		resp, err := c.httpClient.Get(c.baseURL + "/admin/export-json")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var parsed campExport
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("unexpected response (auth required?)")
		}
		return &parsed, nil
	}

	parsed, err := fetch()
	if err != nil {
		// Retry once after logging in (covers session-based auth)
		if loginErr := c.login(); loginErr != nil {
			return nil, err
		}
		parsed, err = fetch()
		if err != nil {
			return nil, err
		}
	}
	// Some deployments omit the "success" field; require it true only when present.
	if (parsed.Success != nil && !*parsed.Success) || parsed.Registrations == nil {
		return nil, fmt.Errorf("camp api reported failure")
	}
	return parsed.Registrations, nil
}

func campField(reg map[string]interface{}, key string) string {
	if v, ok := reg[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

func campBool(reg map[string]interface{}, key string) bool {
	v := strings.ToLower(campField(reg, key))
	return v == "1" || v == "true" || v == "yes"
}

// isAuthorized reports whether the Discord user may query camp data.
// hasAllowedRole is computed by the platform adapter (e.g. Discord role check).
func (c *CampClient) isAuthorized(userID string, hasAllowedRole bool) bool {
	return c != nil && (c.allowedIDs[userID] || hasAllowedRole)
}

// isCampQuery reports whether the message looks like a camp data question.
func isCampQuery(message string) bool {
	m := strings.ToLower(message)
	if strings.HasPrefix(m, "!camp") {
		return true
	}
	if !strings.Contains(m, "camp") {
		return false
	}
	for _, kw := range []string{"regist", "roster", "who", "how many", "signed up", "sign up", "camper", "stats", "attendance", "paid", "unpaid", "capacity", "spots", "full"} {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

// HandleQuery answers camp registration questions directly from the data.
// Returns "" when the message is not a camp query. Unauthorized users get a
// polite refusal so PII is never exposed or sent to an AI provider.
func (c *CampClient) HandleQuery(message, userID string, hasAllowedRole bool) string {
	if c == nil || !isCampQuery(message) {
		return ""
	}
	if !c.isAuthorized(userID, hasAllowedRole) {
		return "🔒 Camp registration data is restricted. Ask a camp administrator for access."
	}

	m := strings.ToLower(strings.TrimSpace(message))
	if m == "!camp" || m == "!camp help" {
		return campHelpMessage()
	}

	regs, err := c.fetchRegistrations()
	if err != nil {
		log.Printf("❌ Camp data fetch failed: %v", err)
		return "⚠️ I couldn't reach the Camp Power-Up registration system right now. Please try again later."
	}

	switch {
	case strings.Contains(m, "unpaid") || strings.Contains(m, "owe") || (strings.Contains(m, "paid") && (strings.Contains(m, "not") || strings.Contains(m, "n't") || strings.Contains(m, "still"))):
		return formatCampUnpaid(regs)
	case strings.Contains(m, "capacity") || strings.Contains(m, "spots") || strings.Contains(m, "full"):
		return formatCampCapacity(regs, c.capacity)
	case strings.Contains(m, "who") || strings.Contains(m, "roster") || strings.Contains(m, "list"):
		return formatCampRoster(regs)
	default:
		return formatCampStats(regs)
	}
}

func campHelpMessage() string {
	return "🏕️ **Camp Power-Up Commands**\n\n" +
		"• `!camp stats` - registration overview\n" +
		"• `!camp roster` - who's registered\n" +
		"• `!camp unpaid` - campers with outstanding payment\n" +
		"• `!camp capacity` - spots filled vs. available\n\n" +
		"You can also just ask naturally, e.g. \"who registered for camp?\""
}

func formatCampRoster(regs []map[string]interface{}) string {
	if len(regs) == 0 {
		return "🏕️ **Camp Power-Up Roster**\n\nNo registrations yet."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🏕️ **Camp Power-Up Roster** (%d registered)\n\n", len(regs))
	for i, reg := range regs {
		first := campField(reg, "child_first_name")
		last := campField(reg, "child_last_name")
		if last != "" {
			last = string([]rune(last)[0]) + "." // last initial only
		}
		grade := campField(reg, "child_grade")
		age := campField(reg, "child_age")

		line := fmt.Sprintf("%d. **%s %s**", i+1, first, last)
		var details []string
		if age != "" {
			details = append(details, "age "+age)
		}
		if grade != "" {
			details = append(details, "grade "+grade)
		}
		if campBool(reg, "is_returning_camper") {
			details = append(details, "returning")
		}
		if pay := campField(reg, "payment_status"); pay != "" {
			details = append(details, "payment: "+pay)
		}
		if len(details) > 0 {
			line += " (" + strings.Join(details, ", ") + ")"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n_Data served directly from Camp Power-Up - not shared with AI providers._")
	return b.String()
}

func formatCampStats(regs []map[string]interface{}) string {
	total := len(regs)
	returning, allergies, paid, ownSwitch := 0, 0, 0, 0
	for _, reg := range regs {
		if campBool(reg, "is_returning_camper") {
			returning++
		}
		if campBool(reg, "has_allergies") {
			allergies++
		}
		if strings.EqualFold(campField(reg, "payment_status"), "paid") {
			paid++
		}
		if campBool(reg, "bringing_own_switch") {
			ownSwitch++
		}
	}

	return fmt.Sprintf("🏕️ **Camp Power-Up Registration Stats**\n"+
		"• Total registered: %d\n"+
		"• Returning campers: %d\n"+
		"• New campers: %d\n"+
		"• Fully paid: %d\n"+
		"• With allergies: %d\n"+
		"• Bringing own Switch: %d\n\n"+
		"Ask \"who registered for camp?\" for the roster.\n"+
		"_Data served directly from Camp Power-Up - not shared with AI providers._",
		total, returning, total-returning, paid, allergies, ownSwitch)
}

func formatCampUnpaid(regs []map[string]interface{}) string {
	var unpaid []string
	for _, reg := range regs {
		status := campField(reg, "payment_status")
		if strings.EqualFold(status, "paid") {
			continue
		}
		first := campField(reg, "child_first_name")
		last := campField(reg, "child_last_name")
		if last != "" {
			last = string([]rune(last)[0]) + "." // last initial only
		}
		if status == "" {
			status = "unknown"
		}
		unpaid = append(unpaid, fmt.Sprintf("• **%s %s** (payment: %s)", first, last, status))
	}

	if len(unpaid) == 0 {
		return fmt.Sprintf("💰 **Camp Power-Up Payments**\n\nAll %d registered campers are fully paid! 🎉", len(regs))
	}
	return fmt.Sprintf("💰 **Camp Power-Up - Outstanding Payments** (%d of %d)\n\n%s\n\n_Data served directly from Camp Power-Up - not shared with AI providers._",
		len(unpaid), len(regs), strings.Join(unpaid, "\n"))
}

func formatCampCapacity(regs []map[string]interface{}, capacity int) string {
	total := len(regs)
	if capacity <= 0 {
		return fmt.Sprintf("🏕️ **Camp Power-Up Capacity**\n\n• Registered: %d\n• Capacity: not configured (set `CAMP_CAPACITY`)\n", total)
	}
	remaining := capacity - total
	if remaining < 0 {
		remaining = 0
	}
	bar := progressBar(total, capacity, 10)
	return fmt.Sprintf("🏕️ **Camp Power-Up Capacity**\n\n%s **%d / %d** spots filled\n• Remaining: %d\n", bar, total, capacity, remaining)
}

// progressBar renders a simple text progress bar like ▓▓▓▓░░░░░░.
func progressBar(current, max, width int) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	filled := current * width / max
	if filled > width {
		filled = width
	}
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}
