package charts

import (
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
)

// pieMaxSlices is the cap for distinct legend rows. Anything beyond this
// is folded into a single "Other" row so the chart stays readable. The
// number is chosen to match the default series palette length.
const pieMaxSlices = 6

// Pie renders a categorical share breakdown. True pie wedges look poor in
// terminal cells, so this implementation renders a sorted legend with
// proportional bars and percentage labels — the same information without
// the visual penalty.
func Pie(p Palette, table *api.InsightTable, cfg api.ChartConfig, size Size) string {
	series, err := ExtractSeries(table, cfg)
	if err != nil {
		return unavailable(p, "pie", "no numeric data")
	}

	slices := buildPieSlices(series.Points)
	if len(slices) == 0 {
		return unavailable(p, "pie", "all values zero")
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

	plotHeight := size.Height - headerLines - footerLines
	if plotHeight < 1 {
		return unavailable(p, "pie", "not enough height")
	}
	if len(slices) > plotHeight {
		// Re-collapse to fit available rows. Always keep at least the
		// top slice and a single "Other" row when collapsing.
		slices = collapseToRows(slices, plotHeight)
	}

	total := 0.0
	for _, s := range slices {
		total += s.Value
	}

	labelWidth := 0
	for _, s := range slices {
		if w := lipgloss.Width(s.Label); w > labelWidth {
			labelWidth = w
		}
	}
	labelWidth = clamp(labelWidth, 1, size.Width/3)

	// Layout per row: <label>  <bar>  <pct>
	pctWidth := 6 // e.g. "100.0%"
	barWidth := size.Width - labelWidth - pctWidth - 2
	if barWidth < 4 {
		return unavailable(p, "pie", "not enough width")
	}

	labelStyle := lipgloss.NewStyle().Foreground(p.Label).Width(labelWidth).Align(lipgloss.Right)
	pctStyle := lipgloss.NewStyle().Foreground(p.Label).Width(pctWidth).Align(lipgloss.Right)

	rows := make([]string, len(slices))
	for i, s := range slices {
		share := s.Value / total
		cells := int(share * float64(barWidth))
		if cells < 1 {
			cells = 1
		}
		if cells > barWidth {
			cells = barWidth
		}
		barStyle := lipgloss.NewStyle().Foreground(p.SeriesColor(i))
		bar := barStyle.Render(strings.Repeat("█", cells))
		pct := formatPercent(share * 100)
		rows[i] = labelStyle.Render(truncate(s.Label, labelWidth)) +
			" " + bar +
			strings.Repeat(" ", barWidth-cells) +
			" " + pctStyle.Render(pct)
	}

	parts := make([]string, 0, 3)
	if header != "" {
		parts = append(parts, header)
	}
	parts = append(parts, strings.Join(rows, "\n"))
	if series.Skipped > 0 {
		parts = append(parts, p.Muted.Render("("+plural(series.Skipped, "row")+" skipped)"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

type pieSlice struct {
	Label string
	Value float64
}

// buildPieSlices converts points to slices, sorts by value desc, drops
// non-positive entries, and collapses anything past pieMaxSlices into an
// "Other" bucket.
func buildPieSlices(pts []Point) []pieSlice {
	out := make([]pieSlice, 0, len(pts))
	for _, p := range pts {
		if p.Y <= 0 {
			continue
		}
		out = append(out, pieSlice{Label: p.Label, Value: p.Y})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Value > out[j].Value
	})
	return collapseToRows(out, pieMaxSlices)
}

// collapseToRows guarantees len(out) <= max, folding the tail into
// "Other" when needed. max < 1 returns the input unchanged.
func collapseToRows(slices []pieSlice, max int) []pieSlice {
	if max < 1 || len(slices) <= max {
		return slices
	}
	keep := slices[:max-1]
	other := pieSlice{Label: "Other"}
	for _, s := range slices[max-1:] {
		other.Value += s.Value
	}
	return append(keep, other)
}

// formatPercent renders n as a percentage. Values >= 10 use no decimals;
// smaller values use one decimal so "1.5%" does not collapse to "1%".
func formatPercent(n float64) string {
	if n >= 10 {
		return formatAxisValue(n) + "%"
	}
	return trimZero(strconv.FormatFloat(n, 'f', 1, 64)) + "%"
}
