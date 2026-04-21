package auth

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
			return ring, nil
		}
	}
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
