package charts

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/api"
)

// Size is the rendering box for a chart in terminal cells. Renderers must
// produce output that fits within these bounds: at most Height lines, each
// line at most Width columns of visible width.
type Size struct {
	Width  int
	Height int
}

// Valid reports whether a Size has both dimensions large enough to attempt
// rendering. The thresholds are intentionally low — individual renderers
// may impose stricter minimums and degrade to an unavailable notice when
// their box is too small.
func (s Size) Valid() bool {
	return s.Width >= 10 && s.Height >= 3
}

// Renderer is the signature every chart implementation satisfies. Returning
// only a string (no error) is deliberate — see the package doc for the
// rationale.
type Renderer func(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string

// Render dispatches to the right renderer for chartType. Unknown types and
// invalid input return an empty string so the caller can fall back to its
// existing badge or skip rendering entirely. chartType is matched
// case-insensitively after trimming whitespace.
//
// Note: "table" and "tile" are intentionally NOT registered here. Those
// types have bespoke rendering paths in internal/ui/components and predate
// this package; routing them through here would force a layout change for
// no benefit.
func Render(chartType string, p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	if !size.Valid() {
		return ""
	}
	if table == nil || len(table.Rows) == 0 {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(chartType))
	renderer, ok := registry[normalized]
	if !ok {
		return ""
	}
	return renderer(p, table, cfg, size)
}

// IsSupported reports whether chartType has a registered renderer in this
// package. Callers use it to decide between calling Render and falling
// back to ChartBadge.
func IsSupported(chartType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(chartType))
	_, ok := registry[normalized]
	return ok
}

// registry is the explicit dispatch table. Listing renderers here (rather
// than self-registering via init() in each chart file) keeps the full set
// of supported chart types greppable in one place and removes any
// import-for-side-effects requirement when this package is split or
// extended. Adding a new chart type is one new file plus one new entry
// in this map.
var registry = map[string]Renderer{
	"bar":     Bar,
	"pie":     Pie,
	"line":    Line,
	"area":    Area,
	"scatter": Scatter,
	"radar":   Radar,
}
