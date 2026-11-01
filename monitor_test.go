package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestWebsiteMonitorRejects404(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
	session, _ := discordgo.New("Bot synthetic")
	var posted string
	session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		posted = string(body)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"test"}`))}, nil
	})
	monitor := NewCampMonitor(camp, session, "", "123", time.Minute)
	monitor.checkWebsite(context.Background())
	if monitor.siteUp || !strings.Contains(posted, "DOWN") {
		t.Fatalf("404 must be down, posted=%s", posted)
	}
}

func TestWebsiteMonitorRedirects(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	for _, sameOrigin := range []bool{true, false} {
		t.Run(fmt.Sprint("sameOrigin=", sameOrigin), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/healthy" {
					w.WriteHeader(http.StatusOK)
					return
				}
				destination := target.URL
				if sameOrigin {
					destination = "/healthy"
				}
				http.Redirect(w, r, destination, http.StatusFound)
			}))
			defer server.Close()
			camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
			monitor := &CampMonitor{camp: camp, haveSite: true, siteUp: sameOrigin}
			monitor.checkWebsite(context.Background())
			if monitor.siteUp != sameOrigin {
				t.Fatalf("siteUp=%v, want %v", monitor.siteUp, sameOrigin)
			}
			if requests.Load() != 0 {
				t.Fatal("website monitor followed a cross-origin redirect")
			}
		})
	}
}

func TestMonitorCancelsInFlightRequest(t *testing.T) {
	for _, website := range []bool{true, false} {
		t.Run(fmt.Sprint("website=", website), func(t *testing.T) {
			started := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				started <- struct{}{}
				<-r.Context().Done()
			}))
			defer server.Close()
			camp := NewCampClient(server.URL, "staff", "test", "123", "", 0)
			session, _ := discordgo.New("Bot synthetic")
			var posts atomic.Int32
			session.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				posts.Add(1)
				return nil, fmt.Errorf("unexpected post during shutdown")
			})
			alerts, status := "123", ""
			if website {
				alerts, status = "", "123"
			}
			monitor := NewCampMonitor(camp, session, alerts, status, time.Minute)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := monitor.Start(ctx)
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("monitor did not start")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("monitor did not cancel its in-flight request")
			}
			if posts.Load() != 0 {
				t.Fatal("monitor posted during shutdown")
			}
		})
	}
}
