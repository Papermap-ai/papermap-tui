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

func (Model) View(th theme.Theme, width int, authenticated bool, workspaceName string) string {
	title := "Terminal-native insights"
	lines := []string{
		"Ask Papermap questions from your terminal.",
		"Sign in with your Papermap account to continue.",
	}
	action := "Press Enter to sign in"
	keyHint := "Enter sign in  •  Ctrl+C quit"

	if authenticated {
		if strings.TrimSpace(workspaceName) == "" {
			workspaceName = "Unified Workspace"
		}

		title = "Welcome back"
		lines = []string{
			"Saved session found.",
			"Continue into " + workspaceName + ".",
		}
		action = "Press Enter to open workspace"
		keyHint = "Enter continue  •  Ctrl+C quit"
	}

	panelWidth := clampWidth(width, 62)
	panel := th.Panel.Width(panelWidth).Render(strings.Join([]string{
		th.Title.Render(title),
		"",
		th.Body.Render(strings.Join(lines, "\n")),
		"",
		th.Accent.Render(action),
		"",
		th.KeyHint.Render(keyHint),
	}, "\n"))

	return strings.Join([]string{
		components.Logo(th),
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
