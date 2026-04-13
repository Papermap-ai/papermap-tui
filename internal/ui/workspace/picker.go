package workspace

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

type Model struct{}

func NewModel() Model {
	return Model{}
}

func (Model) View(th theme.Theme, width int) string {
	return th.Panel.Width(clampWidth(width, 50)).Render(strings.Join([]string{
		th.Title.Render("Switch workspace"),
		"",
		th.Body.Render("> Unified Workspace"),
		th.Muted.Render("  More workspace loading lands in phase 4."),
		"",
		th.KeyHint.Render("Enter select  •  Esc cancel"),
	}, "\n"))
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
