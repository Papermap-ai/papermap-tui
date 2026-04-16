package landing

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

type Model struct{}

func NewModel() Model {
	return Model{}
}

func (Model) View(th theme.Theme, width int) string {
	panelWidth := clampWidth(width, 62)
	panel := th.Panel.Width(panelWidth).Render(strings.Join([]string{
		th.Title.Render("Terminal-native insights"),
		"",
		th.Body.Render(strings.Join([]string{
			"Ask Papermap questions from your terminal.",
			"Sign in with your Papermap account to continue.",
		}, "\n")),
		"",
		th.Accent.Render("Press Enter to sign in"),
		"",
		th.KeyHint.Render("Enter sign in  •  Ctrl+C quit"),
	}, "\n"))

	return strings.Join([]string{
		components.Logo(th, panelWidth),
		th.Muted.Render("Focused terminal access to Papermap insights."),
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
