package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrInsecurePermissions = errors.New("credentials file permissions must be 0600")
	// ErrSessionExpired indicates that the stored credentials have expired and
	// could not be refreshed. The caller should clear state and prompt the
	// user to sign in again.
	ErrSessionExpired = errors.New("session expired")
)

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

type TokenStore struct {
	path      string
	mu        sync.Mutex
	cred      Credentials
	load      bool
	refresher Refresher
}

func NewTokenStore(path string) *TokenStore {
	return &TokenStore{path: path}
}

func DefaultStore() (*TokenStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	return NewTokenStore(filepath.Join(homeDir, ".papermap", "credentials")), nil
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

	cred, err := s.loadLocked()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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
		_ = s.clearLocked()
		return "", ErrSessionExpired
	}

	refreshed, err := s.refresher.Refresh(ctx, cred.RefreshToken)
	if err != nil {
		_ = s.clearLocked()
		return "", fmt.Errorf("%w: %v", ErrSessionExpired, err)
	}

	// Preserve existing user info if the refresh response doesn't include it.
	if refreshed.User == (User{}) {
		refreshed.User = cred.User
	}

	if err := s.saveLocked(refreshed); err != nil {
		return "", err
	}

	return refreshed.AccessToken, nil
}

func (s *TokenStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *TokenStore) loadLocked() (Credentials, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return Credentials{}, err
	}

	if info.Mode().Perm() != 0o600 {
		return Credentials{}, ErrInsecurePermissions
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return Credentials{}, fmt.Errorf("read credentials: %w", err)
	}

	var cred Credentials
	if err := json.Unmarshal(data, &cred); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}

	s.cred = cred
	s.load = true

	return cred, nil
}

func (s *TokenStore) Save(cred Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(cred)
}

func (s *TokenStore) saveLocked(cred Credentials) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	s.cred = cred
	s.load = true

	return nil
}

func (s *TokenStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clearLocked()
}

func (s *TokenStore) clearLocked() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credentials: %w", err)
	}

	s.cred = Credentials{}
	s.load = false

	return nil
}
