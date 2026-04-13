package components

import "github.com/papermap/papermap-tui/internal/theme"

func Logo(th theme.Theme) string {
	return th.Logo.Render("Papermap")
}
