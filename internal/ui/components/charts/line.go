package charts

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
)

// minLineCanvasCols and minLineCanvasRows are the smallest canvas
// (terminal cells) where a line plot still reads as a chart and not as
// noise. Below either threshold the renderer surfaces an unavailable
// notice rather than drawing a degraded version.
const (
	minLineCanvasCols = 10
	minLineCanvasRows = 3
)

// Line renders a single-series line chart using a Braille-cell canvas.
// X is taken from Point.X (which falls back to the row index when the
// payload has no numeric XKey) and Y from Point.Y. The y range is
// extended down to zero when all values are non-negative so the baseline
// stays visible — most analyst payloads (revenue, counts, durations) are
// non-negative and read better with an anchored origin.
func Line(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	series, err := ExtractSeries(table, cfg)
	if err != nil {
		return unavailable(p, "line", "no numeric data")
	}
	return renderBrailleSeries(p, "line", cfg, size, series, false)
}

// renderBrailleSeries is the shared body for line and area. fillBelow
// controls whether each projected y becomes a single dot or fills the
// column down to the baseline.
func renderBrailleSeries(p Palette, kind string, cfg api.ChartConfig, size Size, series Series, fillBelow bool) string {
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
		return unavailable(p, kind, "not enough width")
	}

	// axisFrame adds a bottom rule and an x-tick row, so reserve those
	// lines from the available height.
	canvasRows := size.Height - headerLines - footerLines - 2
	if canvasRows < minLineCanvasRows {
		return unavailable(p, kind, "not enough height")
	}

	xMin, xMax, yMin, yMax, ok := xyBounds(series.Points)
	if !ok {
		return unavailable(p, kind, "no numeric data")
	}
	// Anchor the baseline at zero when the series stays non-negative.
	if yMin >= 0 {
		yMin = 0
	}
	if yMax == yMin {
		// Constant series: pad the range so the projection has room
		// to draw a flat line in the middle of the canvas.
		yMax = yMin + 1
	}

	canvas := newBrailleCanvas(canvasCols, canvasRows)
	dotW := canvasCols * 2
	dotH := canvasRows * 4

	color := p.SeriesColor(0)

	if fillBelow {
		// Area: project each point and fill from the baseline up.
		// The baseline is yMin which projects to dotH-1 at the bottom.
		baselineY := dotH - 1
		for _, pt := range series.Points {
			x := project(pt.X, xMin, xMax, dotW)
			y := dotH - 1 - project(pt.Y, yMin, yMax, dotH)
			canvas.fillColumn(x, y, baselineY)
		}
	}

	// Line/area: connect successive points so the trend reads even when
	// adjacent samples land in non-adjacent canvas columns.
	prevSet := false
	var prevX, prevY int
	for _, pt := range series.Points {
		x := project(pt.X, xMin, xMax, dotW)
		y := dotH - 1 - project(pt.Y, yMin, yMax, dotH)
		if prevSet {
			drawLine(canvas, prevX, prevY, x, y)
		} else {
			canvas.setDot(x, y)
		}
		prevX, prevY, prevSet = x, y, true
	}

	body := axisFrame(canvas, p, color, yMin, yMax, xMin, xMax, xLabelsFromSeries(series))

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

// xLabelsFromSeries returns each point's label so the x-axis tick row
// surfaces categorical context (months, dates, dimensions) rather than
// raw row indices. When labels are uniformly numeric strings the caller
// still gets the same effect — the original column values render in
// place. Returns nil when nothing useful is available.
func xLabelsFromSeries(s Series) []string {
	if len(s.Points) < 2 {
		return nil
	}
	out := make([]string, len(s.Points))
	for i, pt := range s.Points {
		out[i] = pt.Label
	}
	return out
}

// drawLine sets dots along a Bresenham line from (x0, y0) to (x1, y1) so
// successive samples connect cleanly across canvas columns. Without this
// each sample sits as an isolated dot and the chart reads as a sparse
// scatter rather than a continuous trend.
func drawLine(canvas *brailleCanvas, x0, y0, x1, y1 int) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		canvas.setDot(x0, y0)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func buildSeriesFooter(p Palette, skipped int) string {
	if skipped <= 0 {
		return ""
	}
	return p.Muted.Render("(" + plural(skipped, "row") + " skipped)")
}
