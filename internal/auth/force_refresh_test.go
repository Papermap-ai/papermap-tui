package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/auth"
)

type stubRefresher struct {
	refreshToken string
	result       auth.Credentials
	err          error
	called       bool
}

func (s *stubRefresher) Refresh(ctx context.Context, refreshToken string) (auth.Credentials, error) {
	s.called = true
	if s.err != nil {
		return auth.Credentials{}, s.err
	}
	if refreshToken != s.refreshToken {
		return auth.Credentials{}, errors.New("unexpected refresh token")
	}
	return s.result, nil
}

func TestForceRefreshReplacesAccessEvenWhenNotExpired(t *testing.T) {
	t.Parallel()

	store := auth.NewTokenStore(filepath.Join(t.TempDir(), "credentials"))
	existing := auth.Credentials{
		AccessToken:  "still-valid",
		RefreshToken: "rt-1",
		User:         auth.User{UserID: "u-1", Email: "u@x.test"},
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := store.Save(existing); err != nil {
		t.Fatalf("Save: %v", err)
	}

	refreshed := auth.Credentials{
		AccessToken:  "new-access",
		RefreshToken: "rt-2",
		User:         auth.User{UserID: "u-1", Email: "u@x.test"},
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	}
	stub := &stubRefresher{refreshToken: "rt-1", result: refreshed}
	store.SetRefresher(stub)

	got, err := store.ForceRefresh(context.Background())
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}
	if got != "new-access" {
		t.Fatalf("got %q, want new-access", got)
	}
	if !stub.called {
		t.Fatal("expected refresher to be called")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "new-access" || loaded.RefreshToken != "rt-2" {
		t.Fatalf("store not updated: %+v", loaded)
	}
}

func TestForceRefreshClearsOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := auth.NewTokenStore(filepath.Join(dir, "credentials"))
	existing := auth.Credentials{
		AccessToken:  "stale-access",
		RefreshToken: "rt-1",
		ExpiresAt:    time.Now().Add(-time.Minute),
	}
	if err := store.Save(existing); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stub := &stubRefresher{err: errors.New("revoked")}
	store.SetRefresher(stub)

	_, err := store.ForceRefresh(context.Background())
	if err == nil {
		t.Fatal("expected error on refresh failure")
	}
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}

	_, loadErr := store.Load()
	if !errors.Is(loadErr, auth.ErrNoCredentials) {
		t.Fatalf("expected credentials cleared, got %v", loadErr)
	}
}

func TestForceRefreshFailsWithoutRefresher(t *testing.T) {
	t.Parallel()

	store := auth.NewTokenStore(filepath.Join(t.TempDir(), "credentials"))
	if err := store.Save(auth.Credentials{
		AccessToken:  "x",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := store.ForceRefresh(context.Background())
	if err == nil {
		t.Fatal("expected error with no refresher")
	}
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestForceRefreshFailsWithNoCredentials(t *testing.T) {
	t.Parallel()

	store := auth.NewTokenStore(filepath.Join(t.TempDir(), "credentials"))
	stub := &stubRefresher{result: auth.Credentials{AccessToken: "x"}}
	store.SetRefresher(stub)

	_, err := store.ForceRefresh(context.Background())
	if err == nil {
		t.Fatal("expected error with no credentials")
	}
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
	if stub.called {
		t.Fatal("refresher should not be called with no credentials")
	}
}

func TestForceRefreshFailsWithEmptyRefreshToken(t *testing.T) {
	t.Parallel()

	store := auth.NewTokenStore(filepath.Join(t.TempDir(), "credentials"))
	if err := store.Save(auth.Credentials{
		AccessToken:  "x",
		RefreshToken: "",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store.SetRefresher(&stubRefresher{})

	_, err := store.ForceRefresh(context.Background())
	if err == nil || !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "session expired") {
		t.Fatalf("error should mention session expired: %v", err)
	}
}
