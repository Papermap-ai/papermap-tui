package charts

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
)

// Scatter renders a 2D dot plot using a Braille canvas. It requires a
// genuinely numeric x axis: when XKey is missing or non-numeric the
// renderer falls back to Bar with a muted note rather than silently
// plotting row indices, which would be misleading for an analyst trying
// to read a real correlation.
func Scatter(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	series, err := ExtractSeries(table, cfg)
	if err != nil {
		return unavailable(p, "scatter", "no numeric data")
	}

	if !hasNumericXAxis(table, cfg) {
		bar := Bar(p, table, cfg, size)
		if bar == "" {
			return unavailable(p, "scatter", "needs numeric x")
		}
		note := p.Muted.Render("(scatter requires numeric x; rendered as bar)")
		return strings.Join([]string{bar, note}, "\n")
	}

	header := titleBlock(p, cfg)
	headerLines := 0
	if header != "" {
		headerLines = lipgloss.Height(header)
	}

	footerLines := 0
	if series.Skipped > 0 {
		footerLines = 1
	}

	canvasCols := size.Width - axisYTickWidth
	if canvasCols < minLineCanvasCols {
		return unavailable(p, "scatter", "not enough width")
	}

	canvasRows := size.Height - headerLines - footerLines - 2
	if canvasRows < minLineCanvasRows {
		return unavailable(p, "scatter", "not enough height")
	}

	xMin, xMax, yMin, yMax, ok := xyBounds(series.Points)
	if !ok {
		return unavailable(p, "scatter", "no numeric data")
	}
	if yMax == yMin {
		yMax = yMin + 1
	}
	if xMax == xMin {
		xMax = xMin + 1
	}

	canvas := newBrailleCanvas(canvasCols, canvasRows)
	dotW := canvasCols * 2
	dotH := canvasRows * 4

	for _, pt := range series.Points {
		x := project(pt.X, xMin, xMax, dotW)
		y := dotH - 1 - project(pt.Y, yMin, yMax, dotH)
		canvas.setDot(x, y)
	}

	body := axisFrame(canvas, p, p.SeriesColor(0), yMin, yMax, xMin, xMax, nil)

	parts := make([]string, 0, 3)
	if header != "" {
		parts = append(parts, header)
	}
	parts = append(parts, body)
	if footer := buildSeriesFooter(p, series.Skipped); footer != "" {
		parts = append(parts, footer)
	}
	return strings.Join(parts, "\n")
}

// hasNumericXAxis reports whether the configured x column exists and
// holds numeric values. When XKey is empty, the column auto-fallback
// used by ExtractSeries has no semantic meaning for scatter, so this
// returns false and lets the caller route to the Bar fallback.
func hasNumericXAxis(table *api.InsightTable, cfg api.ChartConfig) bool {
	if table == nil || cfg.XKey == "" {
		return false
	}
	idx := -1
	for i, col := range table.Columns {
		if col == cfg.XKey {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	return columnIsNumeric(table, idx)
}
