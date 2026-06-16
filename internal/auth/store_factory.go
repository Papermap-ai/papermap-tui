package auth

import "errors"

// StoreOptions configures NewCredentialStore. The zero value yields the
// production defaults: try the OS keyring first, fall back to the file
// store at the legacy path under $HOME/.papermap/credentials.
type StoreOptions struct {
	// FilePath overrides the on-disk fallback location. When empty the
	// default path under the user's home directory is used.
	FilePath string
	// ForceFile skips the keyring probe and uses the file store directly.
	// Useful for tests, CI, and headless environments where the user
	// already knows the keyring backend will fail.
	ForceFile bool
}

// NewCredentialStore selects an appropriate credential backend. It tries
// the OS keyring first (unless ForceFile is set) and falls back to the
// file store on any failure. The fallback failure modes are diagnosed
// when the file store itself cannot be initialized (e.g. unresolved
// home directory).
func NewCredentialStore(opts StoreOptions) (CredentialStore, error) {
	if !opts.ForceFile {
		if ring, err := newKeyringStore(); err == nil {
			fallback, err := fileFallbackStore(opts)
			if err != nil {
				return nil, err
			}
			return &migratingStore{primary: ring, fallback: fallback}, nil
		}
	}
	return fileFallbackStore(opts)
}

func fileFallbackStore(opts StoreOptions) (CredentialStore, error) {
	path := opts.FilePath
	if path == "" {
		resolved, err := defaultFilePath()
		if err != nil {
			return nil, err
		}
		path = resolved
	}
	return newFileStore(path), nil
}

// migratingStore keeps Keychain as the primary store while still reading the
// old file fallback once. A successful migration deletes the fallback file so
// future launches don't leave stale tokens behind.
type migratingStore struct {
	primary  CredentialStore
	fallback CredentialStore
}

func (s *migratingStore) Kind() StoreKind { return s.primary.Kind() }

func (s *migratingStore) Load() (Credentials, error) {
	cred, err := s.primary.Load()
	if err == nil || !errors.Is(err, ErrNoCredentials) {
		return cred, err
	}

	cred, err = s.fallback.Load()
	if err != nil {
		return Credentials{}, err
	}
	if err := s.primary.Save(cred); err != nil {
		return Credentials{}, err
	}
	_ = s.fallback.Clear()
	return cred, nil
}

func (s *migratingStore) Save(cred Credentials) error {
	if err := s.primary.Save(cred); err != nil {
		return err
	}
	_ = s.fallback.Clear()
	return nil
}

func (s *migratingStore) Clear() error {
	primaryErr := s.primary.Clear()
	fallbackErr := s.fallback.Clear()
	if primaryErr != nil {
		return primaryErr
	}
	return fallbackErr
}
