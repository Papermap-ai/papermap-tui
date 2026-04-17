package components

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// ConfirmDialog renders a compact yes/no modal. Callers own selection state
// and key handling; this is pure presentation.
type ConfirmDialog struct {
	Title   string
	Message string
	Yes     string
	No      string
	// YesSelected reports whether the Yes button is focused.
	YesSelected bool
}

// View renders the dialog as a bordered panel meant to be composited over
// other content via a lipgloss Layer/Compositor.
func (d ConfirmDialog) View(th theme.Theme, screenWidth int) string {
	yesLabel := d.Yes
	if yesLabel == "" {
		yesLabel = "Yep!"
	}
	noLabel := d.No
	if noLabel == "" {
		noLabel = "Nope"
	}

	selectedBtn := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#11111B")).
		Background(th.LogoColorA).
		Bold(true).
		Padding(0, 3)
	unselectedBtn := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F2F5F4")).
		Background(lipgloss.Color("#2A2A35")).
		Padding(0, 3)

	var yesBtn, noBtn string
	if d.YesSelected {
		yesBtn = selectedBtn.Render(yesLabel)
		noBtn = unselectedBtn.Render(noLabel)
	} else {
		yesBtn = unselectedBtn.Render(yesLabel)
		noBtn = selectedBtn.Render(noLabel)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, "  ", noBtn)

	lines := []string{}
	if strings.TrimSpace(d.Title) != "" {
		lines = append(lines, th.Title.Render(d.Title))
	}
	if strings.TrimSpace(d.Message) != "" {
		lines = append(lines, th.Body.Render(d.Message))
	}
	lines = append(lines, "", buttons)

	body := lipgloss.JoinVertical(lipgloss.Center, lines...)

	panelWidth := 52
	if screenWidth > 0 && screenWidth-6 < panelWidth {
		panelWidth = screenWidth - 6
	}
	if panelWidth < 28 {
		panelWidth = 28
	}

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.LogoColorA).
		Padding(1, 3).
		Width(panelWidth).
		Align(lipgloss.Center)

	return panel.Render(body)
}
