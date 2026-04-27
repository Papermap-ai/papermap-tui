// Package palette renders a centered command palette overlay used by the
// chat screen. It is a focused sub-model that emits SelectMsg/CancelMsg
// via its Update method; the parent model is responsible for executing
// the command action.
package palette

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// Command describes one entry in the palette. ID is opaque to the palette
// and is echoed back via SelectMsg so the parent can dispatch the action.
type Command struct {
	ID       string
	Title    string
	Hint     string
	Shortcut string
}

type SelectMsg struct {
	Command Command
}

type CancelMsg struct{}

const (
	maxPanelW = 72
	minPanelW = 48
)

// panelWidthFor scales the palette panel to the terminal, mirroring the
// workspace picker so overlays feel consistent.
func panelWidthFor(screenWidth int) int {
	if screenWidth <= 0 {
		return minPanelW
	}
	width := (screenWidth - 6) * 4 / 5
	if width > maxPanelW {
		width = maxPanelW
	}
	if width < minPanelW {
		width = screenWidth - 6
		if width < 28 {
			width = 28
		}
	}
	return width
}

type Model struct {
	commands []Command
	cursor   int
}

func NewModel() Model {
	return Model{}
}

// SetCommands resets the palette state with the provided commands and
// places the cursor on the first entry.
func (m *Model) SetCommands(commands []Command) {
	m.commands = commands
	m.cursor = 0
}

func (m Model) Commands() []Command {
	return m.commands
}

func (m Model) CurrentCursorIndex() int {
	return m.cursor
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		return m, func() tea.Msg { return CancelMsg{} }

	case "enter":
		if len(m.commands) == 0 {
			return m, nil
		}
		if m.cursor < 0 || m.cursor >= len(m.commands) {
			return m, nil
		}
		selected := m.commands[m.cursor]
		return m, func() tea.Msg { return SelectMsg{Command: selected} }

	case "down", "j", "ctrl+n":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor + 1) % len(m.commands)
		return m, nil

	case "up", "k":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor - 1 + len(m.commands)) % len(m.commands)
		return m, nil

	case "home", "g":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.cursor = 0
		return m, nil

	case "end", "G":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.cursor = len(m.commands) - 1
		return m, nil
	}

	return m, nil
}

func (m Model) View(th theme.Theme, screenWidth int) string {
	width := panelWidthFor(screenWidth)

	header := th.Title.Render("Command palette")

	var body string
	if len(m.commands) == 0 {
		body = th.Muted.Render("No commands available.")
	} else {
		body = m.renderEntries(th, width-8)
	}

	footer := th.KeyHint.Render("↑↓ navigate  •  Enter run  •  Esc cancel")

	content := strings.Join([]string{header, "", body, "", footer}, "\n")

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.LogoColorA).
		Padding(1, 3).
		Width(width)

	return panel.Render(content)
}

func (m Model) renderEntries(th theme.Theme, innerWidth int) string {
	if innerWidth < 20 {
		innerWidth = 20
	}

	cursorStyle := lipgloss.NewStyle().Foreground(th.LogoColorA).Bold(true)

	lines := make([]string, 0, len(m.commands))
	for i, cmd := range m.commands {
		isCursor := i == m.cursor

		var prefix string
		if isCursor {
			prefix = cursorStyle.Render("›")
		} else {
			prefix = " "
		}

		title := cmd.Title
		if isCursor {
			title = th.Accent.Render(title)
		} else {
			title = th.Body.Render(title)
		}

		shortcut := ""
		if strings.TrimSpace(cmd.Shortcut) != "" {
			shortcut = th.Muted.Render(cmd.Shortcut)
		}

		// Two-column layout: title left, shortcut right-aligned.
		left := fmt.Sprintf("%s %s", prefix, title)
		line := alignTwoColumns(left, shortcut, innerWidth)
		lines = append(lines, line)

		if strings.TrimSpace(cmd.Hint) != "" && isCursor {
			lines = append(lines, "  "+th.Muted.Render(cmd.Hint))
		}
	}

	return strings.Join(lines, "\n")
}

// alignTwoColumns places left at column 0 and right at the far right of the
// available width. lipgloss-rendered strings include ANSI escapes, so we
// rely on lipgloss.Width for visible measurement.
func alignTwoColumns(left, right string, width int) string {
	if right == "" {
		return left
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
