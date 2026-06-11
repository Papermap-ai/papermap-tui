package app

import (
	"testing"

	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/ui/chat/modelpicker"
	"github.com/papermap/papermap-tui/internal/ui/components/palette"
	"github.com/papermap/papermap-tui/internal/ui/workspace"
)

func TestPaletteWorkspaceCancelReturnsToPalette(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.authenticated = true
	m.workspaces = []config.WorkspaceEntry{{WorkspaceID: "ws-a", Name: "Alpha"}}
	m.screen = screenCommandPalette

	cmd := m.dispatchPaletteCommand(palette.Command{ID: commandSwitchWorkspace})
	if cmd != nil {
		t.Fatal("expected no refresh cmd without client")
	}
	if m.screen != screenWorkspacePicker {
		t.Fatalf("screen after command = %q, want %q", m.screen, screenWorkspacePicker)
	}

	updated, _ := m.Update(workspace.CancelMsg{})
	got := updated.(Model)
	if got.screen != screenCommandPalette {
		t.Fatalf("screen after workspace cancel = %q, want %q", got.screen, screenCommandPalette)
	}
}

func TestDirectWorkspaceCancelReturnsToChat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.authenticated = true
	m.workspaces = []config.WorkspaceEntry{{WorkspaceID: "ws-a", Name: "Alpha"}}
	m.screen = screenChat

	cmd := m.openWorkspacePicker()
	if cmd != nil {
		t.Fatal("expected no refresh cmd without client")
	}
	updated, _ := m.Update(workspace.CancelMsg{})
	got := updated.(Model)
	if got.screen != screenChat {
		t.Fatalf("screen after direct workspace cancel = %q, want %q", got.screen, screenChat)
	}
}

func TestPaletteModelCancelReturnsToPalette(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.authenticated = true
	m.screen = screenCommandPalette

	cmd := m.dispatchPaletteCommand(palette.Command{ID: commandSwitchModel})
	if cmd != nil {
		t.Fatal("expected no command from switch model")
	}
	if m.screen != screenModelPicker {
		t.Fatalf("screen after command = %q, want %q", m.screen, screenModelPicker)
	}

	updated, _ := m.Update(modelpicker.CancelMsg{})
	got := updated.(Model)
	if got.screen != screenCommandPalette {
		t.Fatalf("screen after model cancel = %q, want %q", got.screen, screenCommandPalette)
	}
}

func TestDirectModelCancelReturnsToChat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.authenticated = true
	m.screen = screenChat

	m.openModelPicker()
	updated, _ := m.Update(modelpicker.CancelMsg{})
	got := updated.(Model)
	if got.screen != screenChat {
		t.Fatalf("screen after direct model cancel = %q, want %q", got.screen, screenChat)
	}
}
