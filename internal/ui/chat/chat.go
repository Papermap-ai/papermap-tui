package chat

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

type Model struct{}

func NewModel() Model {
	return Model{}
}

func (Model) View(th theme.Theme, workspace string, width int) string {
	if workspace == "" {
		workspace = "unified workspace"
	}

	body := th.Panel.Width(clampWidth(width, 72)).Render(strings.Join([]string{
		components.StatusBar(th, "workspace: "+workspace, "stream: pending"),
		"",
		th.Body.Render("Chat scaffold ready."),
		th.Muted.Render("Streaming responses, prompt input, and transcript land in phase 3."),
		"",
		th.KeyHint.Render("Ctrl+W switch workspace  •  Ctrl+L clear chat  •  Ctrl+C quit"),
	}, "\n"))

	return strings.Join([]string{components.Logo(th), "", body}, "\n")
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
