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
		Foreground(th.InputBg).
		Background(th.LogoColorA).
		Bold(true).
		Padding(0, 3)
	unselectedBtn := lipgloss.NewStyle().
		Foreground(th.TextColor).
		Background(th.ButtonBgInactive).
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

	panelWidth := confirmPanelWidth(screenWidth)

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.LogoColorA).
		Padding(1, 3).
		Width(panelWidth).
		Align(lipgloss.Center)

	return panel.Render(body)
}

// confirmPanelWidth scales the confirm dialog with the terminal width while
// keeping it visibly compact. The lower bound (46) fits inside the app's
// minimum supported terminal width (60 cols) once the 6-column outer margin
// is subtracted; the upper bound prevents it from sprawling on wide
// terminals.
func confirmPanelWidth(screenWidth int) int {
	const (
		minW = 46
		maxW = 64
	)
	if screenWidth <= 0 {
		return minW
	}
	// Aim for roughly half the screen so the dialog reads as a focused
	// modal rather than a full-width panel.
	width := (screenWidth - 6) / 2
	if width > maxW {
		width = maxW
	}
	if width < minW {
		width = screenWidth - 6
		if width < 28 {
			width = 28
		}
	}
	return width
}
