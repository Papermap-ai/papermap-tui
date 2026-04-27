package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestListChatHistory(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/chats-history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedQuery = r.URL.RawQuery
		capturedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chats": [
				{
					"llm_data_chat_id": "chat-1",
					"name": "Sales review",
					"dashboard_id": "dash-1",
					"created_at": "2026-04-01T00:00:00Z",
					"modified_at": "2026-04-02T00:00:00Z",
					"latest_user_name": "Alice"
				},
				{
					"llm_data_chat_id": "chat-2",
					"name": "Churn analysis",
					"dashboard_id": "dash-1"
				}
			],
			"total_records": 2,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	page, err := client.ListChatHistory(context.Background(), "dash-1", 1, 10, true)
	if err != nil {
		t.Fatalf("ListChatHistory: %v", err)
	}

	if capturedAuth != "Bearer test-token" {
		t.Fatalf("expected bearer auth, got %q", capturedAuth)
	}
	wantQuery := "dashboard_id=dash-1&exclude_insights=true&page=1&per_page=10"
	if capturedQuery != wantQuery {
		t.Fatalf("unexpected query: got %q want %q", capturedQuery, wantQuery)
	}
	if len(page.Chats) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(page.Chats))
	}
	if page.Chats[0].LLMDataChatID != "chat-1" || page.Chats[0].Name != "Sales review" {
		t.Fatalf("unexpected first chat: %+v", page.Chats[0])
	}
	if page.TotalRecords != 2 || page.TotalPages != 1 {
		t.Fatalf("unexpected totals: %+v", page)
	}
}

func TestListChatHistoryRequiresDashboardID(t *testing.T) {
	t.Parallel()

	client, err := api.NewClient("https://example.test", http.DefaultClient, insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.ListChatHistory(context.Background(), "  ", 1, 10, true); err == nil {
		t.Fatal("expected error for empty dashboard id")
	}
}

func TestListConversations(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"conversations": [
				{
					"llm_data_id": "msg-1",
					"user_query": "How are sales?",
					"text_response": "Sales are up 12%",
					"thoughts": "Looked at orders table"
				},
				{
					"llm_data_id": "msg-2",
					"user_query": "By region?",
					"text_response": "West leads at 40%"
				}
			],
			"total_records": 2,
			"total_pages": 1,
			"branch_parent_id": null
		}`))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	page, err := client.ListConversations(context.Background(), "chat-abc", 1, 4)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}

	if capturedPath != "/api/v1/analytics/chats/chat-abc/conversations" {
		t.Fatalf("unexpected path: %s", capturedPath)
	}
	wantQuery := "page=1&per_page=4"
	if capturedQuery != wantQuery {
		t.Fatalf("unexpected query: got %q want %q", capturedQuery, wantQuery)
	}
	if len(page.Conversations) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(page.Conversations))
	}
	if page.Conversations[0].LLMDataID != "msg-1" || page.Conversations[0].Thoughts == "" {
		t.Fatalf("unexpected first conversation: %+v", page.Conversations[0])
	}
}

func TestListConversationsRequiresChatID(t *testing.T) {
	t.Parallel()

	client, err := api.NewClient("https://example.test", http.DefaultClient, insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.ListConversations(context.Background(), "", 1, 4); err == nil {
		t.Fatal("expected error for empty chat id")
	}
}

func TestListChatHistoryDefaultsPagination(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"chats":[],"total_records":0,"total_pages":0}`))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.ListChatHistory(context.Background(), "dash-1", 0, 0, false); err != nil {
		t.Fatalf("ListChatHistory: %v", err)
	}
	want := "dashboard_id=dash-1&exclude_insights=false&page=1&per_page=10"
	if capturedQuery != want {
		t.Fatalf("unexpected default query: got %q want %q", capturedQuery, want)
	}
}
