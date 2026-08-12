package main

import (
	"fmt"
	"net/url"
	"strings"
)

// validateBaseURL parses and validates an operator-configured base URL,
// requiring an http/https scheme and a host. The returned string is
// reconstructed from the parsed URL, guarding against SSRF via malformed
// configuration values (CWE-918).
func validateBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
