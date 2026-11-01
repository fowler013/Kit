package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestLoadTemplateCustomPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-template.json")
	if err := os.WriteFile(path, []byte(`{"name":"Synthetic template","roles":[],"categories":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := loadTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Name != "Synthetic template" {
		t.Fatalf("unexpected template name: %q", tmpl.Name)
	}
	if _, err := loadTemplate(path + ".missing"); err == nil {
		t.Fatal("missing template must fail")
	}
	if err := os.WriteFile(path, []byte(`invalid JSON`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTemplate(path); err == nil {
		t.Fatal("invalid template must fail")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestApplyQuotesGuildName(t *testing.T) {
	const name = "Synthetic\nforged log\r\x1b[31m"
	body, err := json.Marshal(discordgo.Guild{ID: "123", Name: name})
	if err != nil {
		t.Fatal(err)
	}
	session, err := discordgo.New("Bot synthetic")
	if err != nil {
		t.Fatal(err)
	}
	session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, response := http.StatusOK, string(body)
		if strings.HasSuffix(r.URL.Path, "/roles") {
			status, response = http.StatusForbidden, `{"message":"stop after guild log","code":50013}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(response)),
		}, nil
	})
	var output bytes.Buffer
	original := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(original)
	if err := apply(session, "123", &ServerTemplate{}, false); err == nil {
		t.Fatal("expected mocked roles request to fail")
	}
	got := output.String()
	if strings.Count(got, "\n") != 1 || strings.ContainsAny(strings.TrimSuffix(got, "\n"), "\r\x1b") {
		t.Fatalf("unsafe log output: %q", got)
	}
	if !strings.Contains(got, strconv.Quote(name)) {
		t.Fatalf("guild name was not quoted: %q", got)
	}
}
