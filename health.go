package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

func healthPort(getenv func(string) string) (string, error) {
	port := getenv("PORT")
	if port == "" {
		port = getenv("HTTP_PORT")
	}
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("PORT (or HTTP_PORT) must be an integer from 1 to 65535")
	}
	return strconv.Itoa(n), nil
}

func healthHandler(ready func() bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if !ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, `{"status":"not_ready"}`)
			return
		}
		fmt.Fprintln(w, `{"status":"ready"}`)
	})
	return mux
}

func startHealthServer(port string, ready func() bool) (*http.Server, <-chan error, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		return nil, nil, fmt.Errorf("listen for health checks: %w", err)
	}
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           healthHandler(ready),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(listener)
	}()
	return server, done, nil
}

// probeHealth checks the existing process; it never opens a bot connection.
func probeHealth(ctx context.Context, port string) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("readiness probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bot is not ready (HTTP %d)", resp.StatusCode)
	}
	return nil
}
