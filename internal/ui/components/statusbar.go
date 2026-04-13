package components

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

func StatusBar(th theme.Theme, parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			clean = append(clean, part)
		}
	}

	return th.Status.Render(strings.Join(clean, "  |  "))
}
