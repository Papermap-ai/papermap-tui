// Package modelpicker provides the centered overlay used to switch the
// active LLM model. It mirrors the workspace picker's interaction shape
// (arrow navigation, enter to confirm, esc to cancel) so users do not
// have to learn a new flow.
package modelpicker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/theme"
)

// SelectMsg is emitted by Update when the user confirms a selection.
type SelectMsg struct {
	Model api.ModelChoice
}

// CancelMsg is emitted when the user dismisses the picker.
type CancelMsg struct{}

const (
	pageSize  = 8
	maxPanelW = 96
	minPanelW = 54
)

// panelWidthFor scales the picker panel to the terminal width while
// keeping a consistent outer margin. Mirrors workspace.panelWidthFor so
// the two overlays read as a pair.
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

// Model is a stateful overlay rendered by the app shell.
type Model struct {
	choices     []api.ModelChoice
	currentSlug string
	cursor      int
	page        int
}

func NewModel() Model {
	return Model{}
}

// SetChoices resets the picker with the given model list and positions
// the cursor on currentSlug when present.
func (m *Model) SetChoices(choices []api.ModelChoice, currentSlug string) {
	m.choices = choices
	m.currentSlug = strings.TrimSpace(currentSlug)
	m.cursor = 0
	m.page = 0
	for i, c := range choices {
		if c.Slug == m.currentSlug {
			m.cursor = i
			m.page = i / pageSize
			break
		}
	}
}

func (m Model) totalPages() int {
	if len(m.choices) == 0 {
		return 1
	}
	pages := len(m.choices) / pageSize
	if len(m.choices)%pageSize != 0 {
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
		if len(m.choices) == 0 || m.cursor < 0 || m.cursor >= len(m.choices) {
			return m, nil
		}
		selected := m.choices[m.cursor]
		return m, func() tea.Msg { return SelectMsg{Model: selected} }

	case "down", "j", "ctrl+n":
		if len(m.choices) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor + 1) % len(m.choices)
		m.page = m.cursor / pageSize
		return m, nil

	case "up", "k", "ctrl+p":
		if len(m.choices) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor - 1 + len(m.choices)) % len(m.choices)
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
		if len(m.choices) == 0 {
			return m, nil
		}
		m.cursor = 0
		m.page = 0
		return m, nil

	case "end", "G":
		if len(m.choices) == 0 {
			return m, nil
		}
		m.cursor = len(m.choices) - 1
		m.page = m.totalPages() - 1
		return m, nil
	}

	return m, nil
}

// View renders the modal panel.
func (m Model) View(th theme.Theme, screenWidth int) string {
	width := panelWidthFor(screenWidth)

	header := th.Title.Render("Switch model")

	var body string
	if len(m.choices) == 0 {
		body = th.Muted.Render("No models available.")
	} else {
		body = m.renderEntries(th)
	}

	footer := m.renderFooter(th)

	contentParts := []string{header, ""}
	contentParts = append(contentParts, body)

	// When the user only has a single model entitlement, hint at the
	// upgrade path so the picker explains why it is not interactive.
	if len(m.choices) == 1 {
		contentParts = append(contentParts,
			"",
			th.Muted.Render("Upgrade your plan to access more models."))
	}

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
	start := m.page * pageSize
	end := start + pageSize
	if end > len(m.choices) {
		end = len(m.choices)
	}

	cursorStyle := lipgloss.NewStyle().Foreground(th.LogoColorA).Bold(true)
	currentMarker := lipgloss.NewStyle().Foreground(th.LogoColorB)

	lines := make([]string, 0, end-start)
	lastProvider := ""
	for i := start; i < end; i++ {
		choice := m.choices[i]

		// Provider divider so claude/openai/google groupings read
		// clearly when the list spans more than one provider.
		if choice.Provider != lastProvider {
			if lastProvider != "" {
				lines = append(lines, "")
			}
			lines = append(lines, th.Muted.Render(strings.ToUpper(choice.Provider)))
			lastProvider = choice.Provider
		}

		display := strings.TrimSpace(choice.Display)
		if display == "" {
			display = choice.Slug
		}

		isCursor := i == m.cursor
		isCurrent := choice.Slug == m.currentSlug

		var prefix string
		if isCursor {
			prefix = cursorStyle.Render("›")
		} else {
			prefix = " "
		}

		nameStyled := th.Body.Render(display)
		if isCursor {
			nameStyled = th.Accent.Render(display)
		}

		var suffix string
		if isCurrent {
			suffix = "  " + currentMarker.Render("● current")
		}

		line := fmt.Sprintf("%s %s%s", prefix, nameStyled, suffix)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFooter(th theme.Theme) string {
	hints := "↑↓ navigate  •  Enter select  •  Esc cancel"
	if m.totalPages() > 1 {
		hints = fmt.Sprintf("↑↓ navigate  •  ←→ page %d/%d  •  Enter select  •  Esc cancel",
			m.page+1, m.totalPages())
	}
	if len(m.choices) == 0 {
		hints = "Esc cancel"
	}
	return th.KeyHint.Render(hints)
}
