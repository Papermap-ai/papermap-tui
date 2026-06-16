package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func TestMigratingStoreMovesFallbackCredentialsToKeyring(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "credentials")
	fallback := newFileStore(path)
	primary := &keyringStore{ring: keyring.NewArrayKeyring(nil)}
	store := &migratingStore{primary: primary, fallback: fallback}

	want := Credentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		User:         User{Email: "user@example.com"},
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := fallback.Save(want); err != nil {
		t.Fatalf("save fallback: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != want.AccessToken {
		t.Fatalf("AccessToken = %q, want %q", got.AccessToken, want.AccessToken)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file still exists: %v", err)
	}
	if got, err := primary.Load(); err != nil || got.AccessToken != want.AccessToken {
		t.Fatalf("primary Load = %+v, %v", got, err)
	}
}

func TestMigratingStoreSaveAndClearRemoveFallbackFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "credentials")
	fallback := newFileStore(path)
	primary := &keyringStore{ring: keyring.NewArrayKeyring(nil)}
	store := &migratingStore{primary: primary, fallback: fallback}
	cred := Credentials{AccessToken: "access", ExpiresAt: time.Now().Add(time.Hour)}

	if err := fallback.Save(Credentials{AccessToken: "stale"}); err != nil {
		t.Fatalf("save fallback: %v", err)
	}
	if err := store.Save(cred); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file still exists after Save: %v", err)
	}

	if err := fallback.Save(Credentials{AccessToken: "stale"}); err != nil {
		t.Fatalf("save fallback again: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file still exists after Clear: %v", err)
	}
	if _, err := primary.Load(); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("primary still has credentials: %v", err)
	}
}
