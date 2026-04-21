package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// ErrSessionExpired indicates that the stored credentials have expired and
// could not be refreshed. The caller should clear state and prompt the
// user to sign in again.
var ErrSessionExpired = errors.New("session expired")

// refreshSkew keeps a small safety margin so we refresh slightly before the
// token actually expires. This avoids races where a request starts just
// before expiry and lands after.
const refreshSkew = 60 * time.Second

// Refresher exchanges a refresh token for a fresh set of credentials. It is
// implemented outside this package (typically by the api client) and wired
// into the TokenStore so the store can refresh lazily during AccessToken
// lookups.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (Credentials, error)
}

type User struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	User         User      `json:"user"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (c Credentials) Valid() bool {
	return c.AccessToken != "" && time.Now().Before(c.ExpiresAt)
}

// ValidWithSkew reports whether the access token is still valid after
// subtracting the given safety skew from its expiry. Use this to decide
// whether to refresh preemptively.
func (c Credentials) ValidWithSkew(skew time.Duration) bool {
	return c.AccessToken != "" && time.Now().Add(skew).Before(c.ExpiresAt)
}

// TokenStore is the public auth-state facade. Internally it delegates to
// a CredentialStore (keyring or file fallback) for persistence; callers
// keep using TokenStore so the rest of the app stays decoupled from the
// backend choice.
type TokenStore struct {
	mu        sync.Mutex
	backend   CredentialStore
	refresher Refresher
}

// NewTokenStoreWithBackend builds a TokenStore over an explicit backend.
// Tests use this with an in-memory store; production code uses
// DefaultStore which selects keyring-or-file via the factory.
func NewTokenStoreWithBackend(backend CredentialStore) *TokenStore {
	return &TokenStore{backend: backend}
}

// NewTokenStore preserves the legacy constructor signature: build a
// TokenStore that persists to the given file path. Kept so existing
// callers and tests don't need to change.
func NewTokenStore(path string) *TokenStore {
	return NewTokenStoreWithBackend(newFileStore(path))
}

// DefaultStore returns the production TokenStore: keyring first, file
// fallback under $HOME/.papermap/credentials. Setting the environment
// variable PAPERMAP_FORCE_FILE_STORE=1 skips the keyring probe and uses
// the file backend directly. Useful for tests, CI, and headless or
// sandboxed environments where probing the keyring is undesirable.
func DefaultStore() (*TokenStore, error) {
	backend, err := NewCredentialStore(StoreOptions{
		ForceFile: forceFileFromEnv(),
	})
	if err != nil {
		return nil, err
	}
	return NewTokenStoreWithBackend(backend), nil
}

func forceFileFromEnv() bool {
	switch os.Getenv("PAPERMAP_FORCE_FILE_STORE") {
	case "1", "true", "TRUE", "yes":
		return true
	}
	return false
}

// Kind reports which storage backend the underlying CredentialStore uses.
// Callers can surface this in diagnostics or settings UI.
func (s *TokenStore) Kind() StoreKind {
	return s.backend.Kind()
}

// SetRefresher wires a refresher into the store. Safe to call at any time;
// subsequent AccessToken calls will use it to refresh when needed.
func (s *TokenStore) SetRefresher(r Refresher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresher = r
}

// AccessToken returns a valid access token, refreshing it in place if the
// stored token has expired (or is close to expiring) and a refresh token is
// available. On refresh failure it clears credentials and returns
// ErrSessionExpired so callers can route the user back to login.
func (s *TokenStore) AccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cred, err := s.backend.Load()
	if err != nil {
		if errors.Is(err, ErrNoCredentials) {
			return "", nil
		}
		return "", err
	}

	if cred.ValidWithSkew(refreshSkew) {
		return cred.AccessToken, nil
	}

	// Access token expired (or close to). Try to refresh.
	if s.refresher == nil {
		return cred.AccessToken, nil
	}
	if cred.RefreshToken == "" {
		_ = s.backend.Clear()
		return "", ErrSessionExpired
	}

	refreshed, err := s.refresher.Refresh(ctx, cred.RefreshToken)
	if err != nil {
		_ = s.backend.Clear()
		return "", fmt.Errorf("%w: %v", ErrSessionExpired, err)
	}

	// Preserve existing user info if the refresh response doesn't include it.
	if refreshed.User == (User{}) {
		refreshed.User = cred.User
	}

	if err := s.backend.Save(refreshed); err != nil {
		return "", err
	}

	return refreshed.AccessToken, nil
}

// Load returns the persisted credentials, or the zero value plus
// ErrNoCredentials when nothing is stored. The zero-value-on-missing path
// makes it easy to branch on "are we logged in" without unwrapping.
func (s *TokenStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cred, err := s.backend.Load()
	if err != nil {
		if errors.Is(err, ErrNoCredentials) {
			return Credentials{}, err
		}
		return Credentials{}, err
	}
	return cred, nil
}

// Save persists the given credentials.
func (s *TokenStore) Save(cred Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backend.Save(cred)
}

// Clear removes the persisted credentials. Returns nil when nothing is
// stored, matching the existing logout behavior.
func (s *TokenStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backend.Clear()
}
