package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
)

func TestRefresherDoesNotDeadlockTokenStore(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request api.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode refresh request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.RefreshToken != "old-refresh" {
			t.Errorf("refresh token = %q, want old-refresh", request.RefreshToken)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":     "Token refreshed successfully",
			"success":     true,
			"status_code": http.StatusOK,
			"data": map[string]any{
				"access_token":  jwtForTest(time.Now().Add(time.Hour)),
				"refresh_token": "new-refresh",
				"token_type":    "bearer",
			},
		})
	}))
	defer server.Close()

	store := auth.NewTokenStore(filepath.Join(t.TempDir(), "credentials"))
	wantUser := auth.User{UserID: "user-1", Email: "user@example.com"}
	if err := store.Save(auth.Credentials{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		User:         wantUser,
		ExpiresAt:    time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	client, err := api.NewClient(server.URL, server.Client(), store)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	store.SetRefresher(api.NewRefresher(client))

	done := make(chan error, 1)
	go func() {
		_, err := store.AccessToken(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AccessToken: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AccessToken deadlocked during refresh")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load refreshed credentials: %v", err)
	}
	if got.RefreshToken != "new-refresh" {
		t.Errorf("refresh token = %q, want new-refresh", got.RefreshToken)
	}
	if got.User != wantUser {
		t.Errorf("user = %+v, want %+v", got.User, wantUser)
	}
}
