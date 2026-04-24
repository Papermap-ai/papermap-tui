package charts

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// unavailable returns the standard muted notice used when a chart cannot
// be rendered. reason is a short, lowercase phrase, e.g. "no numeric data".
func unavailable(p Palette, chartType, reason string) string {
	msg := fmt.Sprintf("[%s unavailable: %s]", chartType, reason)
	return p.Muted.Render(msg)
}

// titleBlock renders the chart title and subtitle (when present) above the
// plot area. Either or both may be empty; the function returns "" when
// both are empty so callers can join blocks unconditionally.
func titleBlock(p Palette, cfg ChartTitleSource) string {
	title := strings.TrimSpace(cfg.GetTitle())
	subtitle := strings.TrimSpace(cfg.GetSubtitle())
	if title == "" && subtitle == "" {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(p.Label).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(p.Axis)

	switch {
	case title != "" && subtitle != "":
		return lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(title),
			subtitleStyle.Render(subtitle),
		)
	case title != "":
		return titleStyle.Render(title)
	default:
		return subtitleStyle.Render(subtitle)
	}
}

// ChartTitleSource is the minimum surface titleBlock needs from a config.
// Defining it as an interface keeps titleBlock decoupled from the
// concrete api.ChartConfig and makes it trivial to test.
type ChartTitleSource interface {
	GetTitle() string
	GetSubtitle() string
}

// formatAxisValue renders a float for axis tick labels: integer-valued
// numbers omit the decimal, others use up to 2 fractional digits. Large
// magnitudes are abbreviated with k/M/B suffixes to keep tick labels
// narrow enough for terminal axes.
func formatAxisValue(v float64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}

	switch {
	case abs >= 1e9:
		return trimZero(strconv.FormatFloat(v/1e9, 'f', 1, 64)) + "B"
	case abs >= 1e6:
		return trimZero(strconv.FormatFloat(v/1e6, 'f', 1, 64)) + "M"
	case abs >= 1e3:
		return trimZero(strconv.FormatFloat(v/1e3, 'f', 1, 64)) + "k"
	default:
		// Integer-valued floats render without decimals; otherwise
		// keep up to 2 fractional digits.
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return trimZero(strconv.FormatFloat(v, 'f', 2, 64))
	}
}

// trimZero strips trailing zeros and a trailing decimal point from a
// formatted float so "1.50" -> "1.5" and "2.00" -> "2".
func trimZero(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// truncate clamps s to a maximum visible width by appending "…" when it
// overflows. width <= 0 returns s unchanged. The cut walks runes, not
// bytes, so multibyte labels (city names, customer names, anything
// non-ASCII) are not corrupted mid-codepoint.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "…"
}
