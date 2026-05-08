package components

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

// ChartBadge returns a short dim "[chart: <type>]" indicator for chart
// types the TUI cannot render natively. The native renderers cover bar,
// line, pie, scatter, area, and radar; "table" and "tile" have their
// own bespoke rendering paths in this package. For any of those known
// types and for empty / null values the function returns "" so callers
// can omit the badge entirely. Unknown chart types still get a badge so
// the user is informed that something was emitted but not displayed.
func ChartBadge(th theme.Theme, chartType string) string {
	normalized := strings.ToLower(strings.TrimSpace(chartType))
	switch normalized {
	case "",
		"table", "tile",
		"bar", "line", "pie", "scatter", "area", "radar",
		"<nil>", "null", "none":
		return ""
	}

	return th.Muted.Render("[chart: " + normalized + "]")
}
