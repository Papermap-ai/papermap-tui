package auth

import "github.com/99designs/keyring"

// NewFileStoreForTest exposes the unexported file store so tests can
// construct one with a fixed path. Test-only; only present in test builds.
func NewFileStoreForTest(path string) CredentialStore {
	return newFileStore(path)
}

// NewKeyringStoreForTest wraps an injected keyring.Keyring so tests can
// drive the keyring path with an in-memory backend (keyring.NewArrayKeyring).
// This bypasses newKeyringStore's backend selection so tests don't touch
// the real OS keychain.
func NewKeyringStoreForTest(ring keyring.Keyring) CredentialStore {
	return &keyringStore{ring: ring}
}
