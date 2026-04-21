package auth_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"

	"github.com/papermap/papermap-tui/internal/auth"
)

func TestFileStoreRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")

	store := auth.NewTokenStoreWithBackend(auth.NewFileStoreForTest(path))

	want := auth.Credentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		User:         auth.User{Email: "user@example.com"},
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("unexpected credentials round trip: got %+v want %+v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0o600 permissions, got %v", info.Mode().Perm())
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}

	if _, err := store.Load(); !errors.Is(err, auth.ErrNoCredentials) {
		t.Fatalf("expected ErrNoCredentials after clear, got %v", err)
	}
}

func TestFileStoreReadsLegacyUnenvelopedJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")

	// Pre-existing installs wrote raw Credentials JSON without an envelope.
	// New code must still read those without forcing the user to re-login.
	legacy := []byte(`{"access_token":"legacy","refresh_token":"r","expires_at":"2099-01-01T00:00:00Z"}`)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	store := auth.NewTokenStoreWithBackend(auth.NewFileStoreForTest(path))
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.AccessToken != "legacy" {
		t.Fatalf("expected legacy access token preserved, got %q", got.AccessToken)
	}
}

func TestKeyringStoreRoundTrip(t *testing.T) {
	t.Parallel()

	ring := keyring.NewArrayKeyring(nil)
	store := auth.NewTokenStoreWithBackend(auth.NewKeyringStoreForTest(ring))

	if _, err := store.Load(); !errors.Is(err, auth.ErrNoCredentials) {
		t.Fatalf("expected ErrNoCredentials on empty backend, got %v", err)
	}

	want := auth.Credentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		User:         auth.User{Email: "user@example.com"},
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("unexpected credentials round trip: got %+v want %+v", got, want)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, auth.ErrNoCredentials) {
		t.Fatalf("expected ErrNoCredentials after clear, got %v", err)
	}
	// Clear on an already-empty backend should be a no-op rather than an
	// error, mirroring the file store behavior.
	if err := store.Clear(); err != nil {
		t.Fatalf("expected idempotent Clear, got %v", err)
	}
}

func TestStoreFactoryFallsBackToFileWhenForced(t *testing.T) {
	// No t.Parallel: t.Setenv is incompatible with parallel tests.
	t.Setenv("HOME", t.TempDir())

	store, err := auth.NewCredentialStore(auth.StoreOptions{ForceFile: true})
	if err != nil {
		t.Fatalf("NewCredentialStore: %v", err)
	}
	if got := store.Kind(); got != auth.StoreKindFile {
		t.Fatalf("expected file backend with ForceFile, got %s", got)
	}
}
