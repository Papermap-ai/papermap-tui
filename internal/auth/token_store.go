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

var ErrInsecurePermissions = errors.New("credentials file permissions must be 0600")

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

type TokenStore struct {
	path string
	mu   sync.RWMutex
	cred Credentials
	load bool
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

func (s *TokenStore) AccessToken(context.Context) (string, error) {
	cred, err := s.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	return cred.AccessToken, nil
}

func (s *TokenStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credentials: %w", err)
	}

	s.cred = Credentials{}
	s.load = false

	return nil
}
