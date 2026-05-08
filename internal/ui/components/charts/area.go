package charts

import (
	"github.com/papermap/papermap-tui/internal/api"
)

// Area renders a single-series filled-area chart. It shares the braille
// canvas, axis chrome, and projection logic with Line; the only
// difference is that each projected y fills its column down to the
// baseline, which reads as a solid silhouette of the series.
func Area(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	series, err := ExtractSeries(table, cfg)
	if err != nil {
		return unavailable(p, "area", "no numeric data")
	}
	return renderBrailleSeries(p, "area", cfg, size, series, true)
}
