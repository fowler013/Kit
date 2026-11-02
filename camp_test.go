package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCampSessionAuthentication(t *testing.T) {
	for _, authStatus := range []int{http.StatusFound, http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprint(authStatus), func(t *testing.T) {
			var logins atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/admin/login":
					if r.Method == http.MethodPost {
						logins.Add(1)
						if r.FormValue("username") != "staff" || r.FormValue("password") != "synthetic-password" {
							http.Error(w, "bad credentials", 401)
							return
						}
						http.SetCookie(w, &http.Cookie{Name: "session", Value: "test", Path: "/"})
						http.Redirect(w, r, "/admin", http.StatusFound)
						return
					}
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, "<form>sign in</form>")
				case "/admin":
					fmt.Fprint(w, "signed in")
				case "/admin/export-json":
					if _, err := r.Cookie("session"); err != nil {
						if authStatus == http.StatusFound {
							http.Redirect(w, r, "/admin/login", authStatus)
						} else {
							w.WriteHeader(authStatus)
							fmt.Fprint(w, `{"success":false}`)
						}
						return
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"success":true,"count":1,"registrations":[{"child_first_name":"Synthetic","child_last_name":"Camper","child_age":9,"child_grade":"4"}]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			camp := NewCampClient(server.URL, "staff", "synthetic-password", "123", "", 30)
			for i := 0; i < 2; i++ {
				count, err := camp.RegistrationCount()
				if err != nil || count != 1 {
					t.Fatalf("count=%d error=%v", count, err)
				}
			}
			if logins.Load() != 1 {
				t.Fatalf("session not reused: %d logins", logins.Load())
			}
			if response := camp.HandleQuery("!camp roster", "123", false); !strings.Contains(response, "Synthetic C.") {
				t.Fatalf("unexpected synthetic roster response: %s", response)
			}
		})
	}
}

func TestCampFailureResponses(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"missing deployment", `{"success":true,"registrations":[]}`, 404},
		{"server failure", `{"success":true,"registrations":[]}`, 500},
		{"invalid JSON", `invalid`, 200},
		{"reported failure", `{"success":false,"registrations":[]}`, 200},
		{"missing registrations", `{"success":true}`, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/admin/login" {
					t.Error("non-authentication errors must not trigger login")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
			if _, err := camp.RegistrationCount(); err == nil {
				t.Fatal("failure must not look like zero registrations")
			}
		})
	}
}

func TestCampAccessAndCancellation(t *testing.T) {
	var requests atomic.Int32
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
	if got := camp.HandleQuery("!camp roster", "456", false); !strings.Contains(got, "restricted") {
		t.Fatalf("unauthorized response: %s", got)
	}
	if requests.Load() != 0 {
		t.Fatal("unauthorized user fetched registrations")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := camp.registrationCount(ctx)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation must fail the request")
		}
	case <-time.After(time.Second):
		t.Fatal("camp request did not cancel")
	}
}

func TestCampAllowedChannels(t *testing.T) {
	camp := NewCampClient("https://camp.example", "staff", "test", "123", "", 0)
	if camp.ChannelAllowed("555") {
		t.Fatal("channels must be denied until configured")
	}
	if n := camp.SetAllowedChannels(" 555 , ,777,"); n != 2 {
		t.Fatalf("configured %d channels, want 2", n)
	}
	for channel, want := range map[string]bool{"555": true, "777": true, "999": false, "": false} {
		if got := camp.ChannelAllowed(channel); got != want {
			t.Errorf("ChannelAllowed(%q)=%v, want %v", channel, got, want)
		}
	}
	var missing *CampClient
	if missing.SetAllowedChannels("555") != 0 || missing.ChannelAllowed("555") {
		t.Fatal("nil client must deny every channel")
	}
}

func TestCampRejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "camp.example.com", "file:///etc/passwd", "ftp://camp.example.com", "https:///missing-host"} {
		if camp := NewCampClient(baseURL, "staff", "test", "123", "", 0); camp != nil {
			t.Errorf("accepted invalid base URL %q", baseURL)
		}
	}
}

func TestCampRejectsCrossOriginRedirects(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	for _, path := range []string{"/admin/login", "/admin/export-json"} {
		t.Run(path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer server.Close()
			camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
			var err error
			if path == "/admin/login" {
				err = camp.login(context.Background())
			} else {
				_, err = camp.RegistrationCount()
			}
			if err == nil {
				t.Fatal("cross-origin redirect must fail")
			}
			if requests.Load() != 0 {
				t.Fatal("request followed a cross-origin redirect")
			}
		})
	}
}
