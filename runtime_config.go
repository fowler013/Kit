package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func validateRuntimeConfig(getenv func(string) string) error {
	slackBot, slackApp := getenv("SLACK_BOT_TOKEN"), getenv("SLACK_APP_TOKEN")
	if (slackBot == "") != (slackApp == "") {
		return fmt.Errorf("Slack requires both SLACK_BOT_TOKEN and SLACK_APP_TOKEN; leave both empty for Discord-only mode")
	}
	if slackBot == "" && getenv("DISCORD_BOT_TOKEN") == "" {
		return fmt.Errorf("configure DISCORD_BOT_TOKEN or both Slack tokens")
	}
	for _, key := range []string{"DISCORD_BOT_TOKEN", "SLACK_BOT_TOKEN", "SLACK_APP_TOKEN", "GEMINI_API_KEY", "CLAUDE_API_KEY"} {
		value := getenv(key)
		if strings.TrimSpace(value) != value || strings.HasPrefix(value, "your-") || strings.HasSuffix(value, "-here") {
			return fmt.Errorf("%s contains whitespace or an example placeholder; replace it or leave it empty", key)
		}
	}
	if _, err := healthPort(getenv); err != nil {
		return err
	}
	if (getenv("OPENAI_COMPAT_BASE_URL") == "") != (getenv("OPENAI_COMPAT_MODEL") == "") {
		return fmt.Errorf("set both OPENAI_COMPAT_BASE_URL and OPENAI_COMPAT_MODEL")
	}
	if base := getenv("OPENAI_COMPAT_BASE_URL"); base != "" {
		if _, err := validateBaseURL(base); err != nil {
			return fmt.Errorf("OPENAI_COMPAT_BASE_URL must be an HTTP(S) URL")
		}
	}
	base := getenv("CAMP_API_BASE_URL")
	if base == "" {
		for _, key := range []string{"CAMP_ADMIN_USERNAME", "CAMP_ADMIN_PASSWORD", "CAMP_ALERTS_CHANNEL", "CAMP_STATUS_CHANNEL"} {
			if getenv(key) != "" {
				return fmt.Errorf("%s requires CAMP_API_BASE_URL", key)
			}
		}
		return nil
	}
	parsed, err := url.Parse(base)
	if _, validationErr := validateBaseURL(base); validationErr != nil || err != nil || parsed.User != nil {
		return fmt.Errorf("CAMP_API_BASE_URL must be an HTTP(S) URL without embedded credentials")
	}
	if getenv("CAMP_ADMIN_USERNAME") == "" || getenv("CAMP_ADMIN_PASSWORD") == "" {
		return fmt.Errorf("camp integration requires CAMP_ADMIN_USERNAME and CAMP_ADMIN_PASSWORD")
	}
	if strings.TrimSpace(getenv("CAMP_ALLOWED_DISCORD_IDS")) == "" && strings.TrimSpace(getenv("CAMP_ALLOWED_ROLE")) == "" {
		return fmt.Errorf("camp integration requires CAMP_ALLOWED_DISCORD_IDS or CAMP_ALLOWED_ROLE")
	}
	for _, id := range strings.Split(getenv("CAMP_ALLOWED_DISCORD_IDS"), ",") {
		if id = strings.TrimSpace(id); id != "" && !isAllDigits(id) {
			return fmt.Errorf("CAMP_ALLOWED_DISCORD_IDS must contain comma-separated numeric Discord user IDs")
		}
	}
	for _, setting := range []struct {
		key      string
		min, max int
	}{
		{"CAMP_CAPACITY", 0, 1000000},
		{"CAMP_POLL_MINUTES", 1, 1440},
	} {
		if value := getenv(setting.key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < setting.min || n > setting.max {
				return fmt.Errorf("%s must be an integer from %d to %d", setting.key, setting.min, setting.max)
			}
		}
	}
	return nil
}
