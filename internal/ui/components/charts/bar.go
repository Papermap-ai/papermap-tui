package charts

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
)

// Bar renders a horizontal bar chart. Horizontal orientation is chosen
// because terminal cells are wider than tall, label columns can hold full
// category names without rotation, and value scaling reads top-to-bottom
// like the source table.
//
// Layout per row:
//
//	<label, padded>  <bar of █ chars>  <value>
//
// When the bounding size cannot fit at least the smallest viable layout,
// the function returns the muted unavailable notice rather than a broken
// frame.
func Bar(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	series, err := ExtractSeries(table, cfg)
	if err != nil {
		return unavailable(p, "bar", "no numeric data")
	}

	header := titleBlock(p, cfg)
	headerLines := 0
	if header != "" {
		headerLines = lipgloss.Height(header)
	}

	// Reserve one line for an optional footer when rows were skipped.
	footerLines := 0
	if series.Skipped > 0 {
		footerLines = 1
	}

	plotHeight := size.Height - headerLines - footerLines
	if plotHeight < 1 {
		return unavailable(p, "bar", "not enough height")
	}

	// Cap visible bars to plotHeight so the chart never overflows. When
	// truncated, surface the remainder in the footer.
	points := series.Points
	truncated := 0
	if len(points) > plotHeight {
		truncated = len(points) - plotHeight
		points = points[:plotHeight]
	}

	labelWidth := longestLabelWidth(points)
	labelWidth = clamp(labelWidth, 1, size.Width/3)
	valueStrings := make([]string, len(points))
	maxValueWidth := 0
	for i, pt := range points {
		valueStrings[i] = formatAxisValue(pt.Y)
		if w := lipgloss.Width(valueStrings[i]); w > maxValueWidth {
			maxValueWidth = w
		}
	}

	// Layout: label + " " + bar + " " + value. Two single-space gutters.
	barWidth := size.Width - labelWidth - maxValueWidth - 2
	if barWidth < 4 {
		return unavailable(p, "bar", "not enough width")
	}

	maxAbs := maxAbsY(points)
	if maxAbs == 0 {
		return unavailable(p, "bar", "all values zero")
	}

	labelStyle := lipgloss.NewStyle().Foreground(p.Label).Width(labelWidth).Align(lipgloss.Right)
	valueStyle := lipgloss.NewStyle().Foreground(p.Label).Width(maxValueWidth).Align(lipgloss.Right)

	rows := make([]string, 0, len(points))
	for i, pt := range points {
		barStyle := lipgloss.NewStyle().Foreground(p.SeriesColor(i))
		bar, cells := buildBar(pt.Y, maxAbs, barWidth)
		// Pad the unused bar cells with spaces so the value column
		// always sits on the far right of the row, matching the pie
		// chart layout. Without padding the value drifts next to the
		// bar tip, which reads as ragged/broken.
		gap := strings.Repeat(" ", barWidth-cells)
		rows = append(rows,
			labelStyle.Render(truncate(pt.Label, labelWidth))+
				" "+barStyle.Render(bar)+gap+
				" "+valueStyle.Render(valueStrings[i]),
		)
	}

	parts := make([]string, 0, 3)
	if header != "" {
		parts = append(parts, header)
	}
	parts = append(parts, strings.Join(rows, "\n"))
	if footer := buildBarFooter(p, series.Skipped, truncated); footer != "" {
		parts = append(parts, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// buildBar produces the bar glyph string for a single value and reports
// how many cells were drawn so the caller can pad the remainder. Positive
// values use solid block █; negative values use light shade ░ to make the
// sign visible without colors. A length-zero value renders as a single
// dim tick so the row remains visually present. cells is clamped to
// [0, width] defensively so an upstream NaN/Inf escape can never produce
// a negative `strings.Repeat` count.
func buildBar(value, maxAbs float64, width int) (string, int) {
	if width <= 0 {
		return "", 0
	}
	abs := value
	if abs < 0 {
		abs = -abs
	}
	cells := int((abs / maxAbs) * float64(width))
	if cells < 1 && abs > 0 {
		cells = 1
	}
	if cells < 0 {
		cells = 0
	}
	if cells > width {
		cells = width
	}
	if cells == 0 {
		return "·", 1
	}
	glyph := "█"
	if value < 0 {
		glyph = "░"
	}
	return strings.Repeat(glyph, cells), cells
}

func buildBarFooter(p Palette, skipped, truncated int) string {
	if skipped == 0 && truncated == 0 {
		return ""
	}
	parts := make([]string, 0, 2)
	if truncated > 0 {
		parts = append(parts, plural(truncated, "row")+" hidden")
	}
	if skipped > 0 {
		parts = append(parts, plural(skipped, "row")+" skipped")
	}
	return p.Muted.Render("(" + strings.Join(parts, ", ") + ")")
}

func longestLabelWidth(pts []Point) int {
	w := 0
	for _, p := range pts {
		if lw := lipgloss.Width(p.Label); lw > w {
			w = lw
		}
	}
	if w == 0 {
		w = 1
	}
	return w
}

func maxAbsY(pts []Point) float64 {
	max := 0.0
	for _, p := range pts {
		v := p.Y
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	return max
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
