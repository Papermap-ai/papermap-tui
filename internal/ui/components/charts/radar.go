package charts

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/api"
)

// Radar renders the multi-axis comparison payload that the backend
// labels as "radar". A true polar plot in 50x12 cells reads as noise, so
// we render the data as a horizontal bar group — one bar per axis — and
// disambiguate from a plain "bar" chart by appending a muted hint when
// the payload supplies no subtitle of its own.
//
// The bar rendering reads the same way an analyst would interpret radar
// spokes: each axis gets a labeled row, the value scales across the
// shared maximum, and the comparison stays meaningful at terminal scale.
func Radar(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	bar := Bar(p, table, cfg, size)
	if bar == "" || strings.Contains(bar, "unavailable") {
		return bar
	}
	if strings.TrimSpace(cfg.Subtitle) != "" {
		return bar
	}
	hint := p.Muted.Render("(radar: per-axis values)")
	return bar + "\n" + hint
}
