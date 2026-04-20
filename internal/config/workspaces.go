package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// WorkspaceEntry is the cached representation of a workspace surfaced in the
// switch-workspace picker. Keep this minimal: only fields the UI and the
// switch flow read.
type WorkspaceEntry struct {
	WorkspaceID      string `json:"workspace_id"`
	Name             string `json:"name"`
	WorkspaceType    string `json:"workspace_type"`
	IsUnified        bool   `json:"is_unified"`
	DefaultDashboard string `json:"default_dashboard"`
}

// WorkspaceCache is the on-disk cache written after a successful login or
// session restore.
type WorkspaceCache struct {
	Workspaces []WorkspaceEntry `json:"workspaces"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// IsEmpty reports whether the cache has no entries.
func (c WorkspaceCache) IsEmpty() bool { return len(c.Workspaces) == 0 }

func workspacesPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".papermap", "workspaces.json"), nil
}

// LoadWorkspaces reads the cached workspace list from disk. A missing file
// returns an empty cache and no error.
func LoadWorkspaces() (WorkspaceCache, error) {
	path, err := workspacesPath()
	if err != nil {
		return WorkspaceCache{}, err
	}
	return loadWorkspacesFrom(path)
}

func loadWorkspacesFrom(path string) (WorkspaceCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return WorkspaceCache{}, nil
		}
		return WorkspaceCache{}, fmt.Errorf("read workspaces cache: %w", err)
	}
	if len(data) == 0 {
		return WorkspaceCache{}, nil
	}

	var cache WorkspaceCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return WorkspaceCache{}, fmt.Errorf("decode workspaces cache: %w", err)
	}
	return cache, nil
}

// SaveWorkspaces persists the cache atomically with 0o600 permissions.
func SaveWorkspaces(cache WorkspaceCache) error {
	path, err := workspacesPath()
	if err != nil {
		return err
	}
	return saveWorkspacesTo(path, cache)
}

func saveWorkspacesTo(path string, cache WorkspaceCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if cache.UpdatedAt.IsZero() {
		cache.UpdatedAt = time.Now().UTC()
	}

	payload, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspaces cache: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".workspaces-*.json")
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write workspaces cache: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod workspaces cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close workspaces cache: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace workspaces cache: %w", err)
	}

	return nil
}

// ClearWorkspaces removes the cache file. Missing file is not an error.
func ClearWorkspaces() error {
	path, err := workspacesPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove workspaces cache: %w", err)
	}
	return nil
}
