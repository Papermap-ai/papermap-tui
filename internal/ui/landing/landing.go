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

// View renders the landing screen. When message is empty, the default
// signed-in tagline + prompt is shown. When message is non-empty, it
// replaces the body copy to surface "not signed in" or "session expired"
// guidance pointing the user at `papermap auth login`.
func (Model) View(th theme.Theme, width int, message string) string {
	panelWidth := clampWidth(width, 62)

	centerInPanel := func(rendered string) string {
		return lipgloss.PlaceHorizontal(panelWidth, lipgloss.Center, rendered)
	}

	body := strings.TrimSpace(message)
	var panelLines []string
	if body == "" {
		panelLines = []string{
			centerInPanel(th.Body.Render("You're signed in to Papermap.")),
			"",
			centerInPanel(th.Accent.Render("Press Enter to open your workspace")),
			"",
			centerInPanel(th.KeyHint.Render("Enter open  •  Ctrl+C quit")),
		}
	} else {
		panelLines = []string{
			centerInPanel(th.Body.Render(body)),
			"",
			centerInPanel(th.Accent.Render("Run `papermap auth login` to continue")),
			"",
			centerInPanel(th.KeyHint.Render("Any key quit")),
		}
	}

	panel := strings.Join(panelLines, "\n")

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
