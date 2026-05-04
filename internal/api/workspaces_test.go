package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListWorkspaces_DecodesWrappedEnvelope verifies that ListWorkspaces
// correctly unwraps the standard `{message, success, data: {...}}` envelope
// that the Papermap backend returns from /workspaces/paginate.
func TestListWorkspaces_DecodesWrappedEnvelope(t *testing.T) {
	t.Parallel()

	var requestedPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path+"?"+r.URL.RawQuery)

		if r.URL.Path != "/api/v1/analytics/workspaces/paginate" {
			t.Errorf("expected exact path /api/v1/analytics/workspaces/paginate, got %q (raw query=%q)", r.URL.Path, r.URL.RawQuery)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("page") == "" {
			t.Errorf("expected page query param, got raw query %q", r.URL.RawQuery)
		}

		// Real backend shape, single page of results.
		body := map[string]any{
			"message": "ok",
			"success": true,
			"data": map[string]any{
				"workspaces": []map[string]any{
					{
						"workspace_id":      "ws-unified",
						"name":              "Unified",
						"workspace_type":    "unified",
						"is_unified":        true,
						"default_dashboard": "dash-1",
					},
					{
						"workspace_id":      "ws-source-1",
						"name":              "Source 1",
						"workspace_type":    "source",
						"is_unified":        false,
						"default_dashboard": "dash-2",
					},
				},
				"total_records": 2,
				"total_pages":   1,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	entries, err := client.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d (paths=%v)", len(entries), requestedPaths)
	}
	if entries[0].WorkspaceID != "ws-unified" || !entries[0].IsUnified {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].WorkspaceID != "ws-source-1" {
		t.Errorf("unexpected second entry: %+v", entries[1])
	}

	// Sanity-check that the page query string actually reached the server
	// (this guards against url.URL.String() percent-encoding the '?').
	if len(requestedPaths) == 0 {
		t.Fatal("no requests captured")
	}
	if !strings.Contains(requestedPaths[0], "page=1") {
		t.Errorf("query string missing or malformed: %q", requestedPaths[0])
	}
}

// TestListWorkspaces_PaginatesUntilTotalPages verifies the client keeps
// fetching until total_pages is reached.
func TestListWorkspaces_PaginatesUntilTotalPages(t *testing.T) {
	t.Parallel()

	var pageHits int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageHits++
		page := r.URL.Query().Get("page")

		body := map[string]any{
			"message": "ok",
			"success": true,
			"data": map[string]any{
				"workspaces": []map[string]any{
					{"workspace_id": "ws-" + page, "name": "WS " + page},
				},
				"total_pages": 3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	entries, err := client.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	if pageHits != 3 {
		t.Errorf("expected 3 page requests, got %d", pageHits)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (one per page), got %d", len(entries))
	}
}

// TestCreateWorkspace_PostsBodyAndDecodes verifies that CreateWorkspace
// sends the expected JSON body and unwraps the standard envelope.
func TestCreateWorkspace_PostsBodyAndDecodes(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Workspace created successfully",
			"success": true,
			"data": map[string]any{
				"workspace_id":   "ws_new",
				"name":           "Acme",
				"workspace_type": "POSTGRES",
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := client.CreateWorkspace(context.Background(), CreateWorkspaceRequest{
		Name: "Acme",
		Database: &DatabaseInput{
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

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/analytics/workspaces" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["name"] != "Acme" {
		t.Errorf("body.name = %v", gotBody["name"])
	}
	db, ok := gotBody["database"].(map[string]any)
	if !ok {
		t.Fatalf("body.database missing or wrong shape: %+v", gotBody)
	}
	if db["database_type"] != "POSTGRES" || db["host"] != "db.example.com" {
		t.Errorf("database fields wrong: %+v", db)
	}
	if got == nil || got.WorkspaceID != "ws_new" {
		t.Errorf("decoded summary wrong: %+v", got)
	}
}

func TestCreateWorkspace_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://example.invalid", nil, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.CreateWorkspace(context.Background(), CreateWorkspaceRequest{}); err == nil {
		t.Fatal("expected error for empty request")
	}
	if _, err := client.CreateWorkspace(context.Background(), CreateWorkspaceRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for missing database")
	}
}
