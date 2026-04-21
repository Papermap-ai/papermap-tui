package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestCreateChat(t *testing.T) {
	t.Parallel()

	var authHeader string
	var createRequest api.ChatCreateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/chats" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		authHeader = r.Header.Get("Authorization")
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
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	response, err := client.CreateChat(context.Background(), api.ChatCreateRequest{DashboardID: "dash-123"})
	if err != nil {
		t.Fatalf("CreateChat returned error: %v", err)
	}
	if response.LLMDataChatID != "chat-123" {
		t.Fatalf("unexpected create chat response: %+v", response)
	}
	if createRequest.DashboardID != "dash-123" {
		t.Fatalf("unexpected create chat request: %+v", createRequest)
	}
	if authHeader != "Bearer test-token" {
		t.Fatalf("expected auth header on create chat request, got %q", authHeader)
	}
}
