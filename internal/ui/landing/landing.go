package landing

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

type Model struct{}

func NewModel() Model {
	return Model{}
}

func (Model) View(th theme.Theme, width int) string {
	panelWidth := clampWidth(width, 62)

	centerInPanel := func(rendered string) string {
		return lipgloss.PlaceHorizontal(panelWidth, lipgloss.Center, rendered)
	}

	panel := strings.Join([]string{
		centerInPanel(th.Body.Render("Sign in with your Papermap account to continue.")),
		"",
		centerInPanel(th.Accent.Render("Press Enter to sign in")),
		"",
		centerInPanel(th.KeyHint.Render("Enter sign in  •  Ctrl+C quit")),
	}, "\n")

	tagline := lipgloss.PlaceHorizontal(
		panelWidth,
		lipgloss.Center,
		th.Muted.Render("Focused terminal access to Papermap insights."),
	)

	return strings.Join([]string{
		components.Logo(th, panelWidth),
		tagline,
		"",
		panel,
	}, "\n")
}

func clampWidth(width int, fallback int) int {
	if width <= 0 {
		return fallback
	}
	if width < 40 {
		return width
	}
	if width < fallback {
		return width - 4
	}
	return fallback
}
