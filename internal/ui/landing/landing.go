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

func (Model) View(th theme.Theme, width int, authenticated bool) string {
	message := "Press Enter to sign in and open Papermap."
	if authenticated {
		message = "Session found. Press Enter to continue to workspace."
	}

	panel := th.Panel.Width(clampWidth(width, 54)).Render(strings.Join([]string{
		th.Title.Render("Terminal-native insights"),
		"",
		th.Body.Render(message),
		"",
		th.KeyHint.Render("Enter continue  •  Esc back  •  Ctrl+C quit"),
	}, "\n"))

	return strings.Join([]string{
		components.Logo(th),
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
