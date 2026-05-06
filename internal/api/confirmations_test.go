package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

type confirmationTokenSource struct{}

func (confirmationTokenSource) AccessToken(context.Context) (string, error) {
	return "test-token", nil
}

func TestSubmitConfirmation(t *testing.T) {
	t.Parallel()

	var (
		gotPath    string
		gotMethod  string
		gotAuth    string
		gotRequest api.SubmitConfirmationRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read confirm request: %v", err)
		}
		if err := json.Unmarshal(body, &gotRequest); err != nil {
			t.Fatalf("decode confirm request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":     "ok",
			"success":     true,
			"status_code": http.StatusOK,
			"data": map[string]any{
				"request_id":      gotRequest.RequestID,
				"confirmation_id": gotRequest.ConfirmationID,
				"confirmed":       gotRequest.Confirmed,
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), confirmationTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	resp, err := client.SubmitConfirmation(context.Background(), api.SubmitConfirmationRequest{
		RequestID:      "req-1",
		ConfirmationID: "conf-1",
		Confirmed:      true,
	})
	if err != nil {
		t.Fatalf("SubmitConfirmation returned error: %v", err)
	}

	if gotPath != "/api/v1/analytics/requests/confirm" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotRequest.RequestID != "req-1" || gotRequest.ConfirmationID != "conf-1" || !gotRequest.Confirmed {
		t.Fatalf("unexpected request body: %+v", gotRequest)
	}
	if resp.RequestID != "req-1" || resp.ConfirmationID != "conf-1" || !resp.Confirmed {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSubmitConfirmationValidation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called when ids are missing")
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), confirmationTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if _, err := client.SubmitConfirmation(context.Background(), api.SubmitConfirmationRequest{ConfirmationID: "c"}); err == nil {
		t.Fatal("expected error when request_id is missing")
	}
	if _, err := client.SubmitConfirmation(context.Background(), api.SubmitConfirmationRequest{RequestID: "r"}); err == nil {
		t.Fatal("expected error when confirmation_id is missing")
	}
}
