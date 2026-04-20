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

	for i, entry := range entries {
		if entry.WorkspaceID == m.currentID {
			m.cursor = i
			m.page = i / pageSize
			break
		}
	}
}

// SetLoading toggles the loading-state copy. Called when the workspace list
// hasn't been fetched yet.
func (m *Model) SetLoading(loading bool, message string) {
	m.loading = loading
	if message != "" {
		m.loadMessage = message
	}
}

// Entries returns the current entries for callers that need them.
func (m Model) Entries() []config.WorkspaceEntry { return m.entries }

func (m Model) totalPages() int {
	if len(m.entries) == 0 {
		return 1
	}
	pages := len(m.entries) / pageSize
	if len(m.entries)%pageSize != 0 {
		pages++
	}
	return pages
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
		if m.loading || len(m.entries) == 0 {
			return m, nil
		}
		if m.cursor < 0 || m.cursor >= len(m.entries) {
			return m, nil
		}
		selected := m.entries[m.cursor]
		return m, func() tea.Msg { return SelectMsg{Workspace: selected} }

	case "down", "j", "ctrl+n":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor + 1) % len(m.entries)
		m.page = m.cursor / pageSize
		return m, nil

	case "up", "k", "ctrl+p":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor - 1 + len(m.entries)) % len(m.entries)
		m.page = m.cursor / pageSize
		return m, nil

	case "right", "l", "pgdown", "n":
		if m.totalPages() <= 1 {
			return m, nil
		}
		m.page = (m.page + 1) % m.totalPages()
		m.cursor = m.page * pageSize
		return m, nil

	case "left", "h", "pgup", "p":
		if m.totalPages() <= 1 {
			return m, nil
		}
		m.page = (m.page - 1 + m.totalPages()) % m.totalPages()
		m.cursor = m.page * pageSize
		return m, nil

	case "home", "g":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = 0
		m.page = 0
		return m, nil

	case "end", "G":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = len(m.entries) - 1
		m.page = m.totalPages() - 1
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
	switch {
	case m.loading:
		body = th.Muted.Render(m.loadMessage)
	case len(m.entries) == 0:
		body = th.Muted.Render("No workspaces available yet.")
	default:
		body = m.renderEntries(th, width-6) // account for padding
	}

	footer := m.renderFooter(th)

	contentParts := []string{header, ""}
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

func (m Model) renderEntries(th theme.Theme, innerWidth int) string {
	start := m.page * pageSize
	end := start + pageSize
	if end > len(m.entries) {
		end = len(m.entries)
	}

	cursorStyle := lipgloss.NewStyle().Foreground(th.LogoColorA).Bold(true)
	currentMarker := lipgloss.NewStyle().Foreground(th.LogoColorB)

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := m.entries[i]
		name := entry.Name
		if strings.TrimSpace(name) == "" {
			name = entry.WorkspaceID
		}

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

func (m Model) renderFooter(th theme.Theme) string {
	hints := "↑↓ navigate  •  Enter select  •  Esc cancel"
	if m.totalPages() > 1 {
		hints = fmt.Sprintf("↑↓ navigate  •  ←→ page %d/%d  •  Enter select  •  Esc cancel",
			m.page+1, m.totalPages())
	}
	if m.loading || len(m.entries) == 0 {
		hints = "Esc cancel"
	}
	return th.KeyHint.Render(hints)
}
