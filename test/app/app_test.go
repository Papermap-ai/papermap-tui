package app_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/app"
	"github.com/papermap/papermap-tui/internal/auth"
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

func TestStartupWithValidCredentialsRoutesToWorkspacePicker(t *testing.T) {
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
	if cmd != nil {
		t.Fatal("expected no follow-up command from startup update")
	}

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Switch workspace") {
		t.Fatalf("expected workspace picker view after valid startup, got %q", view)
	}
}

func TestStartupRefreshesExpiredCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/refresh" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

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
	if cmd != nil {
		t.Fatal("expected no follow-up command from startup update")
	}

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Switch workspace") {
		t.Fatalf("expected workspace picker view after refresh, got %q", view)
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
	if cmd != nil {
		t.Fatal("expected no follow-up command from startup update")
	}

	view := updated.(app.Model).View().Content
	if !strings.Contains(view, "Terminal-native insights") {
		t.Fatalf("expected landing view after failed refresh, got %q", view)
	}

	if _, err := os.Stat(filepath.Join(home, ".papermap", "credentials")); !os.IsNotExist(err) {
		t.Fatalf("expected credentials file removed after failed refresh, got err=%v", err)
	}
}

func startupForTest(t *testing.T, model app.Model) any {
	t.Helper()

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected startup command")
	}

	return cmd()
}

func jwtForTest(expiresAt time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(expiresAt.Unix(), 10) + `}`))
	return header + "." + payload + ".signature"
}
