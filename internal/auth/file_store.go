package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ErrInsecurePermissions reports that the credentials file on disk has a
// permission mode the store refuses to read. We refuse rather than auto-fix
// because a non-0600 file usually means another process or a misconfigured
// backup tool can read the credentials.
var ErrInsecurePermissions = errors.New("credentials file permissions must be 0600")

// fileStore persists credentials in a single file under the user's home
// directory. It is the fallback path when no OS keychain is available; it
// enforces 0o600 on the file and 0o700 on the parent directory, and writes
// atomically to avoid leaving a partial file behind on crash.
type fileStore struct {
	mu   sync.Mutex
	path string
}

func newFileStore(path string) *fileStore {
	return &fileStore{path: path}
}

func (s *fileStore) Kind() StoreKind { return StoreKindFile }

// defaultFilePath resolves the legacy file path used by the original
// token store. Keeping the path stable means existing installs keep
// working without manual migration.
func defaultFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".papermap", "credentials"), nil
}

func (s *fileStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, ErrNoCredentials
		}
		return Credentials{}, err
	}

	// File-mode checks are POSIX-specific. On Windows the bits returned by
	// os.Stat don't reflect ACLs, so the check would either always pass or
	// always fail; skip it there and rely on the directory ACL instead.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		return Credentials{}, ErrInsecurePermissions
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return Credentials{}, fmt.Errorf("read credentials: %w", err)
	}

	return decodeEnvelope(data)
}

func (s *fileStore) Save(cred Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := ensureSecureDir(dir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(envelope{Version: envelopeVersion, Data: cred}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	// Write to a temp file in the same directory and rename into place so a
	// crash mid-write can't leave a half-written credentials file. The temp
	// file inherits the same 0o600 mode.
	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("set credentials permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close credentials file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		cleanup()
		return fmt.Errorf("rename credentials: %w", err)
	}
	return nil
}

func (s *fileStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credentials: %w", err)
	}
	return nil
}

// ensureSecureDir creates dir with 0o700 if missing, and tightens the mode
// of an existing directory whose permissions are too loose. Loose dir perms
// can leak the credentials file to other local users.
func ensureSecureDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat credentials directory: %w", err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create credentials directory: %w", err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("credentials parent %s is not a directory", dir)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("tighten credentials directory permissions: %w", err)
		}
	}
	return nil
}

// decodeEnvelope handles both the envelope shape and the legacy raw
// Credentials shape. Returning the unwrapped Credentials lets callers stay
// oblivious to the on-disk schema.
func decodeEnvelope(data []byte) (Credentials, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	if env.Version == 0 {
		var legacy Credentials
		if err := json.Unmarshal(data, &legacy); err != nil {
			return Credentials{}, fmt.Errorf("decode legacy credentials: %w", err)
		}
		return legacy, nil
	}
	if env.Version != envelopeVersion {
		return Credentials{}, fmt.Errorf("unsupported credentials version %d", env.Version)
	}
	return env.Data, nil
}
