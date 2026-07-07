package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
)

// refreshableTokenSource simulates a live token store: it returns a stale
// access token from AccessToken and a fresh one after ForceRefresh is
// called. This drives the 401-retry path in the api client.
type refreshableTokenSource struct {
	initial   string
	refreshed string
	refreshFn func() (string, error)
	called    atomic.Bool
}

func (s *refreshableTokenSource) AccessToken(context.Context) (string, error) {
	return s.initial, nil
}

func (s *refreshableTokenSource) ForceRefresh(context.Context) (string, error) {
	s.called.Store(true)
	if s.refreshFn != nil {
		return s.refreshFn()
	}
	return s.refreshed, nil
}

func TestDoRetriesOn401AfterRefresh(t *testing.T) {
	t.Parallel()

	var (
		auths        []string
		requestCount atomic.Int32
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		auths = append(auths, r.Header.Get("Authorization"))

		if requestCount.Load() == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"token expired"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseEnvelope[map[string]any]{
			Message:    "ok",
			Success:    true,
			StatusCode: http.StatusOK,
			Data: map[string]any{
				"workspace_id":   "ws-1",
				"name":           "Acme",
				"workspace_type": "POSTGRES",
			},
		})
	}))
	defer server.Close()

	ts := &refreshableTokenSource{
		initial:   "stale-token",
		refreshed: "fresh-token",
	}

	client, err := api.NewClient(server.URL, server.Client(), ts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := client.CreateWorkspace(context.Background(), api.CreateWorkspaceRequest{
		Name: "Acme",
		Database: &api.DatabaseInput{
			DatabaseType: "POSTGRES",
			Host:         "db.example.com",
			Port:         5432,
			Name:         "app",
			UserName:     "user",
			Password:     "secret",
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	if got.WorkspaceID != "ws-1" {
		t.Fatalf("unexpected workspace: %+v", got)
	}

	if !ts.called.Load() {
		t.Fatal("expected ForceRefresh to be called on 401")
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected 2 requests (original + retry), got %d", requestCount.Load())
	}
	if len(auths) != 2 {
		t.Fatalf("expected 2 auth headers, got %d", len(auths))
	}
	if auths[0] != "Bearer stale-token" {
		t.Errorf("first request auth: %q, want Bearer stale-token", auths[0])
	}
	if auths[1] != "Bearer fresh-token" {
		t.Errorf("retry request auth: %q, want Bearer fresh-token", auths[1])
	}
}

func TestDoStreamRetriesOn401AfterRefresh(t *testing.T) {
	t.Parallel()

	var (
		auths        []string
		requestCount atomic.Int32
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		auths = append(auths, r.Header.Get("Authorization"))

		if requestCount.Load() == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"token expired"}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: phase_update\n")
		_, _ = io.WriteString(w, `data: {"type":"phase_update","phase":"analyzing","message":"Analyzing...","request_id":"req-1","chat_id":"chat-1"}`)
		_, _ = io.WriteString(w, "\n\n")
		_, _ = io.WriteString(w, "event: complete\n")
		_, _ = io.WriteString(w, `data: {"type":"complete","status":"success","request_id":"req-1","chat_id":"chat-1"}`)
		_, _ = io.WriteString(w, "\n\n")
	}))
	defer server.Close()

	ts := &refreshableTokenSource{
		initial:   "stale-token",
		refreshed: "fresh-token",
	}

	client, err := api.NewClient(server.URL, server.Client(), ts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	stream, err := client.OpenInsightStream(context.Background(), api.InsightStreamRequest{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("OpenInsightStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	if !ts.called.Load() {
		t.Fatal("expected ForceRefresh to be called on 401")
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount.Load())
	}
	if auths[1] != "Bearer fresh-token" {
		t.Errorf("retry auth: %q, want Bearer fresh-token", auths[1])
	}
}

func TestDoReturnsSessionExpiredWhenRefreshFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"token expired"}`))
	}))
	defer server.Close()

	ts := &refreshableTokenSource{
		initial: "stale-token",
		refreshFn: func() (string, error) {
			return "", fmt.Errorf("%w: refresh token revoked", auth.ErrSessionExpired)
		},
	}

	client, err := api.NewClient(server.URL, server.Client(), ts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected error on failed refresh")
	}
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}
