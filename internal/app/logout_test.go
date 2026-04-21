package app_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/app"
	"github.com/papermap/papermap-tui/internal/auth"
)

func TestLogoutNoLocalSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	message, err := app.Logout()
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if message != "No local session found." {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestLogoutClearsCredentialsAfterSuccessfulRemoteLogout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/api/v1/auth/logout" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("expected authorization header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForLogoutTest(time.Now().Add(time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	message, err := app.Logout()
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if message != "Logged out successfully." {
		t.Fatalf("unexpected message: %q", message)
	}
	if !called {
		t.Fatal("expected remote logout to be called")
	}

	if _, err := os.Stat(filepath.Join(home, ".papermap", "credentials")); !os.IsNotExist(err) {
		t.Fatalf("expected credentials file removed, got err=%v", err)
	}
}

func TestLogoutClearsCredentialsWhenRemoteLogoutFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "logout failed"})
	}))
	defer server.Close()

	t.Setenv("PAPERMAP_API_URL", server.URL)

	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore returned error: %v", err)
	}

	if err := store.Save(auth.Credentials{
		AccessToken:  jwtForLogoutTest(time.Now().Add(time.Hour)),
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	message, err := app.Logout()
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if message != "Logged out locally. Remote logout failed." {
		t.Fatalf("unexpected message: %q", message)
	}

	if _, err := os.Stat(filepath.Join(home, ".papermap", "credentials")); !os.IsNotExist(err) {
		t.Fatalf("expected credentials file removed, got err=%v", err)
	}
}

func jwtForLogoutTest(expiresAt time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(expiresAt.Unix(), 10) + `}`))
	return header + "." + payload + ".signature"
}
