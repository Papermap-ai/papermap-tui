package components

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

// ChartBadge returns a short dim "[chart: <type>]" indicator for chart
// types the TUI does not render natively (bar, line, pie, scatter, area,
// radar). For "table", "tile", and empty types it returns "" so callers
// can omit the badge entirely.
func ChartBadge(th theme.Theme, chartType string) string {
	normalized := strings.ToLower(strings.TrimSpace(chartType))
	switch normalized {
	case "", "table", "tile", "<nil>", "null", "none":
		return ""
	}

	return th.Muted.Render("[chart: " + normalized + "]")
}
