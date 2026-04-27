package palette_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components/palette"
)

func sampleCommands() []palette.Command {
	return []palette.Command{
		{ID: "conversations", Title: "Conversations", Hint: "Browse prior chats", Shortcut: "Ctrl+P"},
		{ID: "switch-workspace", Title: "Switch workspace", Shortcut: "Ctrl+W"},
		{ID: "toggle-thinking", Title: "Toggle thinking", Shortcut: "Ctrl+T"},
		{ID: "clear", Title: "Clear / new session", Shortcut: "Ctrl+L"},
		{ID: "quit", Title: "Quit", Shortcut: "Ctrl+C"},
	}
}

func TestPaletteRendersTitleAndCommands(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Command palette") {
		t.Fatalf("expected title in view, got %q", view)
	}
	for _, want := range []string{"Conversations", "Switch workspace", "Quit", "Ctrl+P", "Ctrl+C"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view, got %q", want, view)
		}
	}
}

func TestPaletteEmptyState(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "No commands available") {
		t.Fatalf("expected empty-state copy, got %q", view)
	}
}

func TestPaletteCursorWraps(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	if m.CurrentCursorIndex() != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.CurrentCursorIndex())
	}

	// j moves down.
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	if updated.CurrentCursorIndex() != 1 {
		t.Fatalf("expected cursor 1 after j, got %d", updated.CurrentCursorIndex())
	}

	// k from 0 wraps to last.
	wrapped, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	if wrapped.CurrentCursorIndex() != len(sampleCommands())-1 {
		t.Fatalf("expected cursor wrap to %d, got %d", len(sampleCommands())-1, wrapped.CurrentCursorIndex())
	}
}

func TestPaletteEnterEmitsSelectMsg(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	// Move cursor to second entry.
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	_, cmd := updated.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg, ok := cmd().(palette.SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", cmd())
	}
	if msg.Command.ID != "switch-workspace" {
		t.Fatalf("unexpected selection: %+v", msg.Command)
	}
}

func TestPaletteEscEmitsCancel(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected cancel cmd")
	}
	if _, ok := cmd().(palette.CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestPaletteHomeEndKeys(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	end, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'G', Text: "G"}))
	if end.CurrentCursorIndex() != len(sampleCommands())-1 {
		t.Fatalf("expected cursor at end, got %d", end.CurrentCursorIndex())
	}
	home, _ := end.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	if home.CurrentCursorIndex() != 0 {
		t.Fatalf("expected cursor at start, got %d", home.CurrentCursorIndex())
	}
}

func TestPaletteCursorHintRendersOnlyForCursorRow(t *testing.T) {
	t.Parallel()

	m := palette.NewModel()
	m.SetCommands(sampleCommands())

	view := m.View(theme.Default(), 80)
	// First entry's hint should appear (cursor on it).
	if !strings.Contains(view, "Browse prior chats") {
		t.Fatalf("expected cursor hint, got %q", view)
	}
}
