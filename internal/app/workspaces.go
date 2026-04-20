package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// openWorkspacePicker primes the picker with the latest entries and switches
// the screen. Falls back to a synthesized unified entry when no list has
// been fetched yet.
func (m *Model) openWorkspacePicker() {
	entries := m.workspaces
	if len(entries) == 0 {
		entries = workspaceEntriesFromUnified(m.workspaceID, m.workspaceName, m.defaultDashboard)
	}
	m.workspace.SetWorkspaces(entries, m.workspaceID)
	if len(m.workspaces) == 0 {
		m.workspace.SetLoading(true, "Loading workspaces...")
	}
	m.screen = screenWorkspacePicker
}

// switchWorkspace tears down the current chat session and rebinds app state
// to the selected workspace. The next prompt will create a fresh chat.
func (m Model) switchWorkspace(entry config.WorkspaceEntry) Model {
	m.closeStream()
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
	overlay := m.workspace.View(m.theme, m.width)

	baseW := lipgloss.Width(base)
	baseH := lipgloss.Height(base)
	if baseW <= 0 && m.width > 0 {
		baseW = m.width
	}
	if baseH <= 0 && m.height > 0 {
		baseH = m.height
	}

	ow := lipgloss.Width(overlay)
	oh := lipgloss.Height(overlay)
	x := (baseW - ow) / 2
	y := (baseH - oh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	baseLayer := lipgloss.NewLayer(base).Z(0)
	overlayLayer := lipgloss.NewLayer(overlay).X(x).Y(y).Z(1)
	return lipgloss.NewCompositor(baseLayer, overlayLayer).Render()
}
