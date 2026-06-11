package workspace

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/theme"
)

// SelectMsg is emitted by Update when the user confirms a selection. The app
// model handles the actual workspace switch.
type SelectMsg struct {
	Workspace config.WorkspaceEntry
}

// CancelMsg is emitted when the user dismisses the picker.
type CancelMsg struct{}

const (
	pageSize    = 6
	maxPanelW   = 96
	minPanelW   = 54
	titleMargin = 1
)

// panelWidthFor returns the picker panel width scaled to the current
// terminal width. It keeps a 6-column outer margin so the modal never
// touches the terminal edges, then clamps to [minPanelW, maxPanelW].
// minPanelW is chosen to fit inside the app's minimum supported terminal
// width (60 cols) once outer margins are subtracted.
func panelWidthFor(screenWidth int) int {
	if screenWidth <= 0 {
		return minPanelW
	}
	// Target ~80% of available width so the modal breathes on wide
	// terminals but doesn't sprawl edge to edge.
	width := (screenWidth - 6) * 4 / 5
	if width > maxPanelW {
		width = maxPanelW
	}
	if width < minPanelW {
		// Allow shrinking below minPanelW only when the terminal itself
		// is narrower than the minimum so we still render something.
		width = screenWidth - 6
		if width < 28 {
			width = 28
		}
	}
	return width
}

// Model is a stateful, focused picker rendered as a centered overlay.
type Model struct {
	entries     []config.WorkspaceEntry
	currentID   string
	cursor      int
	page        int
	query       string
	loading     bool
	loadMessage string
}

func NewModel() Model {
	return Model{
		loading:     true,
		loadMessage: "Loading workspaces...",
	}
}

// SetWorkspaces resets the picker state with the given entries and positions
// the cursor on the currently active workspace if present.
func (m *Model) SetWorkspaces(entries []config.WorkspaceEntry, currentID string) {
	m.entries = entries
	m.currentID = strings.TrimSpace(currentID)
	m.loading = false
	m.cursor = 0
	m.page = 0
	m.query = ""

	m.resetCursorForFilter()
}

// SetLoading toggles the loading-state copy. Called when the workspace list
// hasn't been fetched yet.
func (m *Model) SetLoading(loading bool, message string) {
	m.loading = loading
	if message != "" {
		m.loadMessage = message
	}
}

func (m Model) totalPages() int {
	if len(m.filteredEntries()) == 0 {
		return 1
	}
	pages := len(m.filteredEntries()) / pageSize
	if len(m.filteredEntries())%pageSize != 0 {
		pages++
	}
	return pages
}

func (m Model) filteredEntries() []config.WorkspaceEntry {
	query := strings.ToLower(strings.TrimSpace(m.query))
	if query == "" {
		return m.entries
	}

	filtered := make([]config.WorkspaceEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		if strings.Contains(strings.ToLower(displayName(entry)), query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func displayName(entry config.WorkspaceEntry) string {
	name := strings.TrimSpace(entry.Name)
	if name != "" {
		return name
	}
	return entry.WorkspaceID
}

func (m *Model) resetCursorForFilter() {
	entries := m.filteredEntries()
	m.cursor = 0
	m.page = 0
	if len(entries) == 0 {
		return
	}

	for i, entry := range entries {
		if entry.WorkspaceID == m.currentID {
			m.cursor = i
			m.page = i / pageSize
			return
		}
	}
}

func (m *Model) setQuery(query string) {
	m.query = query
	m.resetCursorForFilter()
}

// Update handles key input. Emits SelectMsg or CancelMsg via returned cmd.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		return m, func() tea.Msg { return CancelMsg{} }

	case "enter":
		entries := m.filteredEntries()
		if m.loading || len(entries) == 0 {
			return m, nil
		}
		if m.cursor < 0 || m.cursor >= len(entries) {
			return m, nil
		}
		selected := entries[m.cursor]
		return m, func() tea.Msg { return SelectMsg{Workspace: selected} }

	case "down", "ctrl+n":
		entries := m.filteredEntries()
		if len(entries) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor + 1) % len(entries)
		m.page = m.cursor / pageSize
		return m, nil

	case "up", "ctrl+p":
		entries := m.filteredEntries()
		if len(entries) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor - 1 + len(entries)) % len(entries)
		m.page = m.cursor / pageSize
		return m, nil

	case "right", "pgdown":
		if m.totalPages() <= 1 {
			return m, nil
		}
		m.page = (m.page + 1) % m.totalPages()
		m.cursor = m.page * pageSize
		return m, nil

	case "left", "pgup":
		if m.totalPages() <= 1 {
			return m, nil
		}
		m.page = (m.page - 1 + m.totalPages()) % m.totalPages()
		m.cursor = m.page * pageSize
		return m, nil

	case "home":
		if len(m.filteredEntries()) == 0 {
			return m, nil
		}
		m.cursor = 0
		m.page = 0
		return m, nil

	case "end":
		entries := m.filteredEntries()
		if len(entries) == 0 {
			return m, nil
		}
		m.cursor = len(entries) - 1
		m.page = m.totalPages() - 1
		return m, nil

	case "backspace":
		if m.query == "" {
			return m, nil
		}
		runes := []rune(m.query)
		m.setQuery(string(runes[:len(runes)-1]))
		return m, nil

	case "ctrl+u":
		if m.query == "" {
			return m, nil
		}
		m.setQuery("")
		return m, nil
	}

	if keyMsg.Text != "" && keyMsg.Mod == 0 {
		m.setQuery(m.query + keyMsg.Text)
		return m, nil
	}

	return m, nil
}

// View renders the modal panel. screenWidth caps the width so it fits
// gracefully on narrow terminals.
func (m Model) View(th theme.Theme, screenWidth int) string {
	width := panelWidthFor(screenWidth)

	header := th.Title.Render("Switch workspace")

	var body string
	filtered := m.filteredEntries()
	switch {
	case m.loading:
		body = th.Muted.Render(m.loadMessage)
	case len(m.entries) == 0:
		body = th.Muted.Render("No workspaces available yet.")
	case len(filtered) == 0:
		body = th.Muted.Render(fmt.Sprintf("No workspaces match %q.", m.query))
	default:
		body = m.renderEntries(th)
	}

	footer := m.renderFooter(th)

	contentParts := []string{header, "", m.renderSearch(th), ""}
	contentParts = append(contentParts, body)
	if footer != "" {
		contentParts = append(contentParts, "", footer)
	}

	content := strings.Join(contentParts, "\n")

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.LogoColorA).
		Padding(1, 3).
		Width(width)

	return panel.Render(content)
}

func (m Model) renderEntries(th theme.Theme) string {
	entries := m.filteredEntries()
	start := m.page * pageSize
	end := start + pageSize
	if end > len(entries) {
		end = len(entries)
	}

	cursorStyle := lipgloss.NewStyle().Foreground(th.LogoColorA).Bold(true)
	currentMarker := lipgloss.NewStyle().Foreground(th.LogoColorB)

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := entries[i]
		name := displayName(entry)

		isCursor := i == m.cursor
		isCurrent := entry.WorkspaceID == m.currentID

		var prefix string
		if isCursor {
			prefix = cursorStyle.Render("›")
		} else {
			prefix = " "
		}

		nameStyled := th.Body.Render(name)
		if isCursor {
			nameStyled = th.Accent.Render(name)
		}

		var suffix string
		if isCurrent {
			suffix = "  " + currentMarker.Render("● current")
		} else if entry.IsUnified {
			suffix = "  " + th.Muted.Render("unified")
		}

		line := fmt.Sprintf("%s %s%s", prefix, nameStyled, suffix)
		lines = append(lines, line)
	}

	// Pad to consistent height to keep modal stable across pages.
	for len(lines) < pageSize {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderSearch(th theme.Theme) string {
	label := th.Muted.Render("Search: ")
	if m.query == "" {
		return label + th.Muted.Render("type workspace name...")
	}
	return label + th.Accent.Render(m.query)
}

func (m Model) renderFooter(th theme.Theme) string {
	hints := "type to search  •  ↑↓ navigate  •  Enter select  •  Ctrl+U clear  •  Esc cancel"
	if m.totalPages() > 1 {
		hints = fmt.Sprintf("type to search  •  ↑↓ navigate  •  ←→ page %d/%d  •  Enter select  •  Ctrl+U clear  •  Esc cancel",
			m.page+1, m.totalPages())
	}
	if m.loading || len(m.entries) == 0 {
		hints = "Esc cancel"
	} else if len(m.filteredEntries()) == 0 {
		hints = "type to search  •  Ctrl+U clear  •  Esc cancel"
	}
	return th.KeyHint.Render(hints)
}
