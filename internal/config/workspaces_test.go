package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/papermap/papermap-tui/internal/config"
)

func TestWorkspacesRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if cache, err := config.LoadWorkspaces(); err != nil || !cache.IsEmpty() {
		t.Fatalf("expected empty cache for missing file, got cache=%+v err=%v", cache, err)
	}

	entries := []config.WorkspaceEntry{
		{WorkspaceID: "ws-1", Name: "Alpha", IsUnified: true, DefaultDashboard: "d1"},
		{WorkspaceID: "ws-2", Name: "Beta"},
	}
	if err := config.SaveWorkspaces(config.WorkspaceCache{Workspaces: entries}); err != nil {
		t.Fatalf("SaveWorkspaces returned error: %v", err)
	}

	loaded, err := config.LoadWorkspaces()
	if err != nil {
		t.Fatalf("LoadWorkspaces returned error: %v", err)
	}
	if len(loaded.Workspaces) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Workspaces))
	}
	if loaded.Workspaces[0].WorkspaceID != "ws-1" || loaded.Workspaces[1].Name != "Beta" {
		t.Fatalf("entries not preserved, got %+v", loaded.Workspaces)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}

	if runtime.GOOS != "windows" {
		path := filepath.Join(home, ".papermap", "workspaces.json")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat workspaces file: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected mode 0o600, got %o", perm)
		}
	}
}

func TestClearWorkspacesRemovesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := config.SaveWorkspaces(config.WorkspaceCache{
		Workspaces: []config.WorkspaceEntry{{WorkspaceID: "ws-1", Name: "Alpha"}},
	}); err != nil {
		t.Fatalf("SaveWorkspaces returned error: %v", err)
	}

	if err := config.ClearWorkspaces(); err != nil {
		t.Fatalf("ClearWorkspaces returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".papermap", "workspaces.json")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got err=%v", err)
	}

	// Second call on missing file should not error.
	if err := config.ClearWorkspaces(); err != nil {
		t.Fatalf("ClearWorkspaces on missing file returned error: %v", err)
	}
}
