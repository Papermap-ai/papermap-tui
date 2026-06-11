package workspace_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/workspace"
)

func TestPickerLoadingState(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Switch workspace") {
		t.Fatalf("expected title in view, got %q", view)
	}
	if !strings.Contains(view, "Loading workspaces") {
		t.Fatalf("expected loading copy, got %q", view)
	}
}

func TestPickerNavigationCyclesEntries(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	entries := []config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
		{WorkspaceID: "ws-b", Name: "Beta"},
		{WorkspaceID: "ws-c", Name: "Gamma"},
	}
	m.SetWorkspaces(entries, "ws-b")

	// Cursor lands on the current workspace.
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Beta") || !strings.Contains(view, "current") {
		t.Fatalf("expected current marker on Beta, got %q", view)
	}

	// Down should move to Gamma.
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated
	view = m.View(theme.Default(), 80)
	// Cursor indicator on Gamma now (uses ›).
	if !strings.Contains(view, "Gamma") {
		t.Fatalf("expected Gamma in view, got %q", view)
	}
}

func TestPickerSearchFiltersByName(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	entries := []config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
		{WorkspaceID: "ws-b", Name: "Beta"},
		{WorkspaceID: "ws-c", Name: "Gamma"},
	}
	m.SetWorkspaces(entries, "")

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	m = updated
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	m = updated

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Search:") || !strings.Contains(view, "ga") {
		t.Fatalf("expected search query in view, got %q", view)
	}
	if !strings.Contains(view, "Gamma") {
		t.Fatalf("expected Gamma in filtered view, got %q", view)
	}
	if strings.Contains(view, "Alpha") || strings.Contains(view, "Beta") {
		t.Fatalf("expected non-matching workspaces hidden, got %q", view)
	}

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg, ok := cmd().(workspace.SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if msg.Workspace.WorkspaceID != "ws-c" {
		t.Fatalf("expected filtered Gamma selection, got %+v", msg.Workspace)
	}
}

func TestPickerSearchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
		{WorkspaceID: "ws-b", Name: "Beta"},
	}, "")

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'A', Text: "A"}))
	m = updated
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'L', Text: "L"}))
	m = updated

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Alpha") || strings.Contains(view, "Beta") {
		t.Fatalf("expected case-insensitive Alpha match only, got %q", view)
	}
}

func TestPickerSearchFallsBackToWorkspaceID(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{
		{WorkspaceID: "finance-prod"},
		{WorkspaceID: "ws-b", Name: "Marketing"},
	}, "")

	for _, key := range []string{"f", "i", "n"} {
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key}))
		m = updated
	}

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "finance-prod") || strings.Contains(view, "Marketing") {
		t.Fatalf("expected workspace ID fallback match only, got %q", view)
	}
}

func TestPickerSearchBackspaceAndClear(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
		{WorkspaceID: "ws-b", Name: "Beta"},
	}, "")

	for _, key := range []string{"b", "e"} {
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key}))
		m = updated
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	m = updated

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Beta") || strings.Contains(view, "Alpha") {
		t.Fatalf("expected backspace to leave query b and match Beta only, got %q", view)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	m = updated
	view = m.View(theme.Default(), 80)
	if !strings.Contains(view, "Alpha") || !strings.Contains(view, "Beta") {
		t.Fatalf("expected ctrl+u to clear search, got %q", view)
	}
}

func TestPickerJTypesIntoSearch(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
		{WorkspaceID: "ws-b", Name: "Beta"},
	}, "")

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	m = updated
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, `No workspaces match "j".`) {
		t.Fatalf("expected j to type into search, got %q", view)
	}
}

func TestPickerNoSearchResults(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
	}, "")

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}))
	m = updated
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, `No workspaces match "z".`) {
		t.Fatalf("expected no-results copy, got %q", view)
	}
}

func TestPickerEnterEmitsSelectMsg(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	entries := []config.WorkspaceEntry{
		{WorkspaceID: "ws-a", Name: "Alpha"},
	}
	m.SetWorkspaces(entries, "ws-a")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg, ok := cmd().(workspace.SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", cmd())
	}
	if msg.Workspace.WorkspaceID != "ws-a" {
		t.Fatalf("unexpected selection: %+v", msg.Workspace)
	}
}

func TestPickerEscEmitsCancel(t *testing.T) {
	t.Parallel()

	m := workspace.NewModel()
	m.SetWorkspaces([]config.WorkspaceEntry{{WorkspaceID: "ws-a", Name: "Alpha"}}, "ws-a")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected cancel cmd")
	}
	if _, ok := cmd().(workspace.CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestPickerPaginationFooter(t *testing.T) {
	t.Parallel()

	entries := make([]config.WorkspaceEntry, 0, 14)
	for i := 0; i < 14; i++ {
		entries = append(entries, config.WorkspaceEntry{WorkspaceID: "ws", Name: "Workspace"})
	}
	m := workspace.NewModel()
	m.SetWorkspaces(entries, "")

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "page 1/3") {
		t.Fatalf("expected pagination footer, got %q", view)
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	m = updated
	view = m.View(theme.Default(), 80)
	if !strings.Contains(view, "page 2/3") {
		t.Fatalf("expected page 2/3 after right, got %q", view)
	}
}
