package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/99designs/keyring"
)

// keyringServiceName is the logical service identifier we register with the
// OS keychain. Keep it stable; changing it strands existing users' tokens.
const keyringServiceName = "papermap"

// keyringItemKey is the entry name inside the keyring service. Today we
// only store one envelope; the key is plural-agnostic so we can extend
// later without migration.
const keyringItemKey = "credentials"

// keyringStore persists credentials in an OS-managed secret store via
// 99designs/keyring. The file backend is intentionally excluded so the
// fallback path is exercised explicitly via fileStore instead of
// silently dropping into a less-protected on-disk file.
type keyringStore struct {
	mu   sync.Mutex
	ring keyring.Keyring
}

// newKeyringStore opens the OS keyring for the papermap service. If no
// supported backend is available it returns an error so the factory can
// fall back to the file store.
func newKeyringStore() (*keyringStore, error) {
	ring, err := openKeyring(keyring.Config{
		ServiceName: keyringServiceName,
		// Exclude the file backend; we manage our own fallback path with
		// stricter permission checks and clearer diagnostics.
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.WinCredBackend,
			keyring.KWalletBackend,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}
	return &keyringStore{ring: ring}, nil
}

// openKeyring is the seam tests use to inject an in-memory backend. The
// production path delegates straight to keyring.Open.
var openKeyring = keyring.Open

func (s *keyringStore) Kind() StoreKind { return StoreKindKeyring }

func (s *keyringStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, err := s.ring.Get(keyringItemKey)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return Credentials{}, ErrNoCredentials
		}
		return Credentials{}, fmt.Errorf("read keyring entry: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(item.Data, &env); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	if env.Version == 0 {
		// Legacy raw Credentials without an envelope. Keep reading them so
		// users upgrading in place don't lose their session.
		var legacy Credentials
		if err := json.Unmarshal(item.Data, &legacy); err != nil {
			return Credentials{}, fmt.Errorf("decode legacy credentials: %w", err)
		}
		return legacy, nil
	}
	if env.Version != envelopeVersion {
		return Credentials{}, fmt.Errorf("unsupported credentials version %d", env.Version)
	}
	return env.Data, nil
}

func (s *keyringStore) Save(cred Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(envelope{Version: envelopeVersion, Data: cred})
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	item := keyring.Item{
		Key:         keyringItemKey,
		Data:        data,
		Label:       "Papermap CLI session",
		Description: "Access and refresh tokens for the Papermap TUI.",
	}
	if err := s.ring.Set(item); err != nil {
		return fmt.Errorf("write keyring entry: %w", err)
	}
	return nil
}

func (s *keyringStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ring.Remove(keyringItemKey); err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("remove keyring entry: %w", err)
	}
	return nil
}
