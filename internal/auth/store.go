package auth

import "errors"

// StoreKind identifies which backend a CredentialStore is using. The app
// surfaces this via diagnostics and logs so users can tell whether their
// credentials live in the OS keychain or in a fallback file.
type StoreKind string

const (
	// StoreKindKeyring means credentials live in an OS-managed secrets
	// backend (macOS Keychain, Windows Credential Manager, libsecret, ...).
	StoreKindKeyring StoreKind = "keyring"
	// StoreKindFile means credentials live in an on-disk file with strict
	// permissions. Used as a fallback in headless or unsupported envs.
	StoreKindFile StoreKind = "file"
)

// ErrNoCredentials reports that no credentials are stored. It is returned
// by Load when the backend has no entry for the configured key. Callers
// should treat this as the unauthenticated state, not an error to surface.
var ErrNoCredentials = errors.New("no credentials stored")

// envelopeVersion identifies the on-wire schema for stored credentials.
// Bump when the persisted shape changes in a backwards-incompatible way so
// older clients can detect and refuse unknown payloads.
const envelopeVersion = 1

// envelope wraps Credentials with a version tag so future schema changes
// can be detected without breaking existing installs.
type envelope struct {
	Version int         `json:"version"`
	Data    Credentials `json:"data"`
}

// CredentialStore is the storage boundary for persisted auth credentials.
// Implementations must be safe for concurrent use; the TokenStore facade
// relies on this for its lock-free fast paths.
type CredentialStore interface {
	// Load returns the stored credentials, or ErrNoCredentials if nothing
	// is stored. Other errors indicate a storage failure (corrupt data,
	// permission problems, backend unavailable).
	Load() (Credentials, error)
	// Save persists the given credentials, replacing any existing entry.
	Save(cred Credentials) error
	// Clear removes the stored credentials. Returns nil if nothing was
	// stored; only real removal failures surface as errors.
	Clear() error
	// Kind reports which backend is providing storage.
	Kind() StoreKind
}
