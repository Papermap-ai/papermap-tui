package auth

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

type Model struct{}

func NewModel() Model {
	return Model{}
}

func (Model) View(th theme.Theme, width int) string {
	return th.Panel.Width(clampWidth(width, 58)).Render(strings.Join([]string{
		th.Title.Render("Sign in"),
		"",
		th.Body.Render("Email/password flow scaffolding ready."),
		th.Muted.Render("Interactive inputs land in phase 2."),
		"",
		th.KeyHint.Render("Esc back  •  Ctrl+C quit"),
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
