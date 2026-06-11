package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/config"
)

// workspacesLoadedMsg carries the result of a backend workspace fetch and
// is consumed by the root model to update in-memory state and persist the
// cache.
type workspacesLoadedMsg struct {
	entries []config.WorkspaceEntry
	err     error
}

// workspaceCacheMaxAge controls when a restored session triggers a
// background refresh of the cached workspace list.
const workspaceCacheMaxAge = 24 * time.Hour

func loadWorkspacesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return workspacesLoadedMsg{}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		summaries, err := client.ListWorkspaces(ctx)
		if err != nil {
			return workspacesLoadedMsg{err: err}
		}

		entries := make([]config.WorkspaceEntry, 0, len(summaries))
		for _, summary := range summaries {
			entries = append(entries, config.WorkspaceEntry{
				WorkspaceID:      summary.WorkspaceID,
				Name:             summary.Name,
				WorkspaceType:    summary.WorkspaceType,
				IsUnified:        summary.IsUnified,
				DefaultDashboard: summary.DefaultDashboard,
			})
		}

		return workspacesLoadedMsg{entries: entries}
	}
}

// workspaceEntriesFromUnified returns a single-entry slice derived from the
// active unified workspace. Used as a fallback when the cache is empty.
func workspaceEntriesFromUnified(workspaceID, name, defaultDashboard string) []config.WorkspaceEntry {
	if workspaceID == "" {
		return nil
	}
	displayName := name
	if displayName == "" {
		displayName = "Unified Workspace"
	}
	return []config.WorkspaceEntry{{
		WorkspaceID:      workspaceID,
		Name:             displayName,
		IsUnified:        true,
		DefaultDashboard: defaultDashboard,
	}}
}

// shouldRefreshWorkspaces reports whether the cached workspace list is empty
// or older than the freshness threshold.
func (m Model) shouldRefreshWorkspaces() bool {
	if m.client == nil {
		return false
	}
	if len(m.workspaces) == 0 {
		return true
	}
	if m.workspacesAt.IsZero() {
		return true
	}
	return time.Since(m.workspacesAt) > workspaceCacheMaxAge
}

// openWorkspacePicker primes the picker with cached entries and switches the
// screen. It always returns a refresh command so externally-created
// workspaces show up when the picker opens.
func (m *Model) openWorkspacePicker() tea.Cmd {
	entries := m.workspaces
	if len(entries) == 0 {
		entries = workspaceEntriesFromUnified(m.workspaceID, m.workspaceName, m.defaultDashboard)
	}
	m.workspace.SetWorkspaces(entries, m.workspaceID)
	if len(m.workspaces) == 0 {
		m.workspace.SetLoading(true, "Loading workspaces...")
	}
	m.screen = screenWorkspacePicker
	if m.client == nil {
		return nil
	}
	return loadWorkspacesCmd(m.client)
}

// switchWorkspace tears down the current chat session and rebinds app state
// to the selected workspace. The next prompt will create a fresh chat.
// Re-selecting the active workspace is a no-op so the in-memory session
// survives a round-trip through the picker.
func (m Model) switchWorkspace(entry config.WorkspaceEntry) Model {
	if entry.WorkspaceID != "" && entry.WorkspaceID == m.workspaceID {
		m.screen = screenChat
		return m
	}
	m.cancelInsight()
	m.resetInsightState()
	m.chat.Clear()
	m.workspaceID = entry.WorkspaceID
	if name := entry.Name; name != "" {
		m.workspaceName = name
	}
	m.defaultDashboard = entry.DefaultDashboard
	m.screen = screenChat
	return m
}

// overlayWorkspacePicker composites the picker modal centered on the chat
// view, mirroring the quit dialog overlay behavior.
func (m Model) overlayWorkspacePicker(base string) string {
	return m.centerOverlay(base, m.workspace.View(m.theme, m.width))
}
