package auth_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/auth"
)

func TestTokenStoreSaveLoadClear(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials")
	store := auth.NewTokenStore(storePath)

	want := auth.Credentials{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		User: auth.User{
			UserID:    "user-1",
			Email:     "user@example.com",
			FirstName: "Test",
			LastName:  "User",
		},
		ExpiresAt: time.Unix(1_900_000_000, 0).UTC(),
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file mode: %v", info.Mode().Perm())
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected credentials: got %+v want %+v", got, want)
	}

	accessToken, err := store.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken returned error: %v", err)
	}
	if accessToken != want.AccessToken {
		t.Fatalf("unexpected access token: got %q want %q", accessToken, want.AccessToken)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}

	if _, err := os.Stat(storePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected credentials file removed, got err=%v", err)
	}

	accessToken, err = store.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken after clear returned error: %v", err)
	}
	if accessToken != "" {
		t.Fatalf("expected empty access token after clear, got %q", accessToken)
	}
}

func TestTokenStoreLoadRejectsInsecurePermissions(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials")
	store := auth.NewTokenStore(storePath)

	data := []byte(`{"access_token":"access-token"}`)
	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		t.Fatalf("write credentials file: %v", err)
	}

	_, err := store.Load()
	if !errors.Is(err, auth.ErrInsecurePermissions) {
		t.Fatalf("expected ErrInsecurePermissions, got %v", err)
	}
}

func TestCredentialsValid(t *testing.T) {
	t.Parallel()

	if (auth.Credentials{}).Valid() {
		t.Fatal("expected empty credentials to be invalid")
	}

	valid := auth.Credentials{
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	if !valid.Valid() {
		t.Fatal("expected future-dated credentials to be valid")
	}

	expired := auth.Credentials{
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(-10 * time.Minute),
	}
	if expired.Valid() {
		t.Fatal("expected expired credentials to be invalid")
	}
}
