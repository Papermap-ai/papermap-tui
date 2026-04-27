package app_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/app"
	"github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

type responseEnvelope[T any] struct {
	Message    string `json:"message"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Data       T      `json:"data"`
}

func TestNewModelStartsOnLanding(t *testing.T) {
	t.Parallel()

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	if got := model.View().Content; got == "" {
		t.Fatal("expected non-empty initial view")
	}
}

func TestStartupWithValidCredentialsRoutesToChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/analytics/workspaces/unified":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"workspace": map[string]any{
					"workspace_id":           "unified-123",
					"name":                   "Kwabena's Unified Space",
					"workspace_type":         "unified",
					"is_unified":             true,
					"included_workspace_ids": []string{"ws-a"},
				},
			})
		case "/api/v1/analytics/workspaces/unified-123/included-workspaces":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":    true,
				"workspaces": []map[string]any{},
				"settings": map[string]any{
					"workspace_id":            "unified-123",
					"workspace_name":          "Kwabena's Unified Space",
					"included_workspace_ids":  []string{"ws-a"},
					"all_workspaces_included": false,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("PAPERMAP_API_URL", server.URL)

	t.Setenv("HOME", t.TempDir())
	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForTest(time.Now().Add(time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, cmd := model.Update(startupForTest(t, model))
	// Drain any follow-up command (e.g. workspace pagination) without
	// executing it; tests scope assertions to the synchronous startup result.
	_ = cmd

	view := updated.(app.Model).View().Content
	// Workspace label only appears on the chat screen; landing has no
	// per-workspace heading, so this confirms we routed past landing.
	if !strings.Contains(view, "Workspace: Kwabena's Unified Space") {
		t.Fatalf("expected chat empty-state with workspace label after valid startup, got %q", view)
	}
}

func TestStartupBestEffortWhenIncludedWorkspacesFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PAPERMAP_API_URL", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/analytics/workspaces/unified":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"workspace": map[string]any{
					"workspace_id":   "unified-123",
					"name":           "Unified Workspace",
					"workspace_type": "unified",
					"is_unified":     true,
				},
			})
		case "/api/v1/analytics/workspaces/unified-123/included-workspaces":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForTest(time.Now().Add(time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, cmd := model.Update(startupForTest(t, model))
	// Drain any follow-up command (e.g. workspace pagination) without
	// executing it; tests scope assertions to the synchronous startup result.
	_ = cmd

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Unified Workspace") {
		t.Fatalf("expected workspace name even when included-workspaces fails, got %q", view)
	}
}

func TestStartupRefreshesExpiredCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			_ = json.NewEncoder(w).Encode(responseEnvelope[api.AuthTokens]{
				Message:    "Token refreshed successfully",
				Success:    true,
				StatusCode: http.StatusOK,
				Data: api.AuthTokens{
					AccessToken:  jwtForTest(time.Now().Add(2 * time.Hour)),
					RefreshToken: "refresh-token-2",
					TokenType:    "bearer",
					User:         auth.User{Email: "user@example.com"},
				},
			})
		case "/api/v1/analytics/workspaces/unified":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"workspace": map[string]any{
					"workspace_id":           "unified-123",
					"name":                   "Kwabena's Unified Space",
					"workspace_type":         "unified",
					"is_unified":             true,
					"included_workspace_ids": []string{"ws-a", "ws-b"},
				},
			})
		case "/api/v1/analytics/workspaces/unified-123/included-workspaces":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"workspaces": []map[string]any{{
					"workspace_id":   "ws-a",
					"name":           "Workspace A",
					"workspace_type": "csv",
					"included":       true,
				}},
				"settings": map[string]any{
					"workspace_id":            "unified-123",
					"workspace_name":          "Kwabena's Unified Space",
					"included_workspace_ids":  []string{"ws-a", "ws-b"},
					"all_workspaces_included": false,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	oldCred := auth.Credentials{
		AccessToken:  jwtForTest(time.Now().Add(-time.Hour)),
		RefreshToken: "refresh-token",
		User:         auth.User{Email: "user@example.com"},
		ExpiresAt:    time.Now().Add(-time.Hour),
	}
	if err := store.Save(oldCred); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, cmd := model.Update(startupForTest(t, model))
	// Drain any follow-up command (e.g. workspace pagination) without
	// executing it; tests scope assertions to the synchronous startup result.
	_ = cmd

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Workspace: Kwabena's Unified Space") {
		t.Fatalf("expected chat empty-state with workspace label after refresh, got %q", view)
	}

	updatedCred, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if updatedCred.RefreshToken != "refresh-token-2" {
		t.Fatalf("expected refreshed token saved, got %+v", updatedCred)
	}
	if !updatedCred.Valid() {
		t.Fatalf("expected refreshed credentials to be valid, got %+v", updatedCred)
	}
	if updatedCred.User.Email != "user@example.com" {
		t.Fatalf("expected user preserved, got %+v", updatedCred.User)
	}

	if _, err := os.Stat(filepath.Join(home, ".papermap", "credentials")); err != nil {
		t.Fatalf("expected credentials file to remain: %v", err)
	}
}

func TestStartupClearsExpiredCredentialsWhenRefreshFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid refresh token"}`))
	}))
	defer server.Close()

	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForTest(time.Now().Add(-time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, cmd := model.Update(startupForTest(t, model))
	// Drain any follow-up command (e.g. workspace pagination) without
	// executing it; tests scope assertions to the synchronous startup result.
	_ = cmd

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Focused terminal access to Papermap Data Platform") {
		t.Fatalf("expected landing view after failed refresh, got %q", view)
	}

	if _, err := os.Stat(filepath.Join(home, ".papermap", "credentials")); !os.IsNotExist(err) {
		t.Fatalf("expected credentials file removed after failed refresh, got err=%v", err)
	}
}

func TestSubmitCreatesChatBeforeStartingInsight(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var createCalls int
	var streamCalls int
	var createRequest api.ChatCreateRequest
	var chartRequest api.InsightRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/analytics/workspaces/unified":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"workspace": map[string]any{
					"workspace_id":      "unified-123",
					"name":              "Unified Workspace",
					"workspace_type":    "unified",
					"is_unified":        true,
					"default_dashboard": "dash-123",
				},
			})
		case "/api/v1/analytics/workspaces/unified-123/included-workspaces":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":    true,
				"workspaces": []map[string]any{},
				"settings": map[string]any{
					"workspace_id":            "unified-123",
					"workspace_name":          "Unified Workspace",
					"included_workspace_ids":  []string{},
					"all_workspaces_included": false,
				},
			})
		case "/api/v1/analytics/chats":
			createCalls++
			if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
				t.Fatalf("decode create chat request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"llm_data_chat_id": "chat-123",
					"dashboard_id":     "dash-123",
				},
			})
		case "/api/v1/analytics/charts/stream":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read chart request: %v", err)
			}
			if err := json.Unmarshal(body, &chartRequest); err != nil {
				t.Fatalf("decode chart request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(responseEnvelope[map[string]any]{
				Message:    "ok",
				Success:    true,
				StatusCode: http.StatusOK,
				Data: map[string]any{
					"llm_data_id":   "llm-123",
					"response_type": "text",
					"text_response": "final response",
					"status":        "success",
				},
			})
		case "/api/v1/analytics/requests/stream":
			streamCalls++
			w.Header().Set("Content-Type", "text/event-stream")
			if streamCalls == 1 {
				_, _ = io.WriteString(w, strings.Join([]string{
					"event: error",
					`data: {"message":"Request not found","request_id":"req-race"}`,
					"",
				}, "\n"))
				return
			}
			_, _ = io.WriteString(w, strings.Join([]string{
				"event: chunk",
				`data: {"type":"chunk","text":"hello","request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: done",
				`data: {"type":"done","request_id":"req-123","chat_id":"chat-123","done":true}`,
				"",
			}, "\n"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForTest(time.Now().Add(time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, cmd := model.Update(startupForTest(t, model))
	// Drain any follow-up command (e.g. workspace pagination) without
	// executing it; tests scope assertions to the synchronous startup result.
	_ = cmd

	startedModel := updated.(app.Model)
	updated, cmd = startedModel.Update(chat.SubmitMsg{Prompt: "Show revenue"})
	if cmd == nil {
		t.Fatal("expected submit to start insight command")
	}

	_, _ = updated.(app.Model).Update(cmd())

	if createCalls != 1 {
		t.Fatalf("expected create chat called once, got %d", createCalls)
	}
	if createRequest.DashboardID != "dash-123" {
		t.Fatalf("unexpected create chat request: %+v", createRequest)
	}
	if chartRequest.ChatID != "chat-123" {
		t.Fatalf("expected chart request to use created chat id, got %+v", chartRequest)
	}
	if chartRequest.WorkspaceID != "unified-123" {
		t.Fatalf("expected chart request to use workspace id, got %+v", chartRequest)
	}
	if streamCalls < 2 {
		t.Fatalf("expected stream retry after request race, got %d calls", streamCalls)
	}
}

func TestMouseWheelDownScrollsChatViewport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	// Build a transcript tall enough to scroll. Each message renders on
	// multiple lines, so a few dozen entries comfortably exceed an 80x24
	// viewport.
	messages := make([]chat.Message, 0, 60)
	for i := 0; i < 30; i++ {
		messages = append(messages,
			chat.Message{Role: "you", Content: "question " + strconv.Itoa(i)},
			chat.Message{
				Role: "alan",
				Content: "answer line one\nanswer line two\n" +
					"answer line three for message " + strconv.Itoa(i),
			},
		)
	}

	seeded := model.SeedChatForTest(80, 24, messages...)

	// Sanity: transcript must exceed visible height, otherwise scrolling
	// is a no-op and the assertion would be meaningless.
	if total := seeded.Chat().ViewportTotalLines(); total <= 24 {
		t.Fatalf("expected transcript taller than viewport, got %d lines", total)
	}

	beforeOffset := seeded.Chat().ViewportYOffset()
	if beforeOffset != 0 {
		t.Fatalf("expected viewport at top before wheel test, got offset %d", beforeOffset)
	}

	wheel := tea.MouseWheelMsg{Button: tea.MouseWheelDown}
	next, _ := seeded.Update(wheel)
	nextModel, ok := next.(app.Model)
	if !ok {
		t.Fatalf("expected app.Model after Update, got %T", next)
	}

	afterOffset := nextModel.Chat().ViewportYOffset()
	if afterOffset <= beforeOffset {
		t.Fatalf("expected wheel-down to advance viewport offset, before=%d after=%d",
			beforeOffset, afterOffset)
	}
}

func startupForTest(t *testing.T, model app.Model) any {
	t.Helper()

	msg, ok := app.StartupMsgFromInit(model)
	if !ok {
		t.Fatal("expected startupMsg from Init")
	}
	return msg
}

func jwtForTest(expiresAt time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(expiresAt.Unix(), 10) + `}`))
	return header + "." + payload + ".signature"
}
