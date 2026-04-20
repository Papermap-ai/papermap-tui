package components

import (
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// TileFormat describes how a raw tile value should be rendered. Fields are
// optional; zero values fall back to plain numeric or string rendering.
type TileFormat struct {
	// Kind is one of "currency", "percent", "integer", "float", "bool",
	// or "" for auto-detect from the value.
	Kind string
	// CurrencySymbol prefixes currency values. Defaults to "$".
	CurrencySymbol string
	// Decimals controls float/percent/currency decimal places. The zero
	// value means "use the format default" (2 for currency, 1 for
	// percent, 2 for float). To force zero decimals, use Kind="integer"
	// or explicitly set DecimalsSet alongside Decimals.
	Decimals    int
	DecimalsSet bool
}

// TileFormatFromConfig derives a TileFormat from a visualization_config map
// supplied by the backend. Recognized keys: "format", "currency",
// "decimals". Unknown keys are ignored.
func TileFormatFromConfig(cfg map[string]any) TileFormat {
	f := TileFormat{}
	if cfg == nil {
		return f
	}

	if v, ok := cfg["format"].(string); ok {
		f.Kind = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := cfg["currency"].(string); ok {
		f.CurrencySymbol = strings.TrimSpace(v)
	}
	if v, ok := cfg["decimals"].(float64); ok {
		f.Decimals = int(v)
		f.DecimalsSet = true
	}
	return f
}

// RenderTile renders a single-metric card with the label dim+upper above
// the bold value, surrounded by a rounded border. width is a soft cap; the
// card sizes to the value's width when smaller. Returns the empty string
// when label and value are both empty.
func RenderTile(th theme.Theme, width int, label string, value string, format TileFormat) string {
	display := formatTileValue(value, format)
	displayLabel := strings.ToUpper(prettyLabel(label))

	if strings.TrimSpace(display) == "" && strings.TrimSpace(displayLabel) == "" {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(th.Muted.GetForeground()).
		Bold(false)
	valueStyle := lipgloss.NewStyle().
		Foreground(th.Accent.GetForeground()).
		Bold(true)

	body := lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Render(displayLabel),
		valueStyle.Render(display),
	)

	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.Accent.GetForeground()).
		Padding(0, 2)

	// Cap to width when the natural card would overflow. Account for
	// 2 border columns + 4 padding columns = 6 chrome columns; the
	// remaining space is the content width.
	if width > 0 {
		bodyWidth := lipgloss.Width(body)
		maxBodyWidth := width - 6
		if maxBodyWidth < 1 {
			maxBodyWidth = 1
		}
		if bodyWidth > maxBodyWidth {
			border = border.Width(width - 2)
		}
	}

	return border.Render(body)
}

// prettyLabel converts snake_case / kebab-case keys into a more readable
// form by replacing separators with spaces. Casing is left untouched here;
// callers apply their own upper/lower transformation.
func prettyLabel(label string) string {
	if label == "" {
		return ""
	}
	replaced := strings.NewReplacer("_", " ", "-", " ").Replace(label)
	return strings.Join(strings.Fields(replaced), " ")
}

// formatTileValue renders a raw scalar string according to the TileFormat.
// Non-numeric values are returned unchanged when format kind doesn't apply.
func formatTileValue(raw string, format TileFormat) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	switch format.Kind {
	case "bool":
		switch strings.ToLower(trimmed) {
		case "true", "1", "yes":
			return "Yes"
		case "false", "0", "no":
			return "No"
		}
		return trimmed

	case "currency":
		number, ok := parseFloat(trimmed)
		if !ok {
			return trimmed
		}
		symbol := format.CurrencySymbol
		if symbol == "" {
			symbol = "$"
		}
		decimals := 2
		if format.DecimalsSet {
			decimals = format.Decimals
		}
		return symbol + formatNumber(number, decimals)

	case "percent":
		number, ok := parseFloat(trimmed)
		if !ok {
			return trimmed
		}
		decimals := 1
		if format.DecimalsSet {
			decimals = format.Decimals
		}
		return formatNumber(number, decimals) + "%"

	case "integer":
		number, ok := parseFloat(trimmed)
		if !ok {
			return trimmed
		}
		return formatNumber(math.Trunc(number), 0)

	case "float":
		number, ok := parseFloat(trimmed)
		if !ok {
			return trimmed
		}
		decimals := 2
		if format.DecimalsSet {
			decimals = format.Decimals
		}
		return formatNumber(number, decimals)
	}

	// Auto: numeric values get thousands separators, integers stay
	// integers, others render as-is.
	if number, ok := parseFloat(trimmed); ok {
		if number == math.Trunc(number) {
			return formatNumber(number, 0)
		}
		return formatNumber(number, -1)
	}
	return trimmed
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// formatNumber renders n with thousands separators. decimals < 0 keeps the
// natural decimal portion (using strconv.FormatFloat with -1).
func formatNumber(n float64, decimals int) string {
	var formatted string
	if decimals < 0 {
		formatted = strconv.FormatFloat(n, 'f', -1, 64)
	} else {
		formatted = strconv.FormatFloat(n, 'f', decimals, 64)
	}

	negative := strings.HasPrefix(formatted, "-")
	if negative {
		formatted = formatted[1:]
	}

	intPart, fracPart, hasFrac := strings.Cut(formatted, ".")
	withSep := insertThousandsSep(intPart)

	if hasFrac {
		withSep = withSep + "." + fracPart
	}
	if negative {
		withSep = "-" + withSep
	}
	return withSep
}

func insertThousandsSep(intPart string) string {
	n := len(intPart)
	if n <= 3 {
		return intPart
	}
	first := n % 3
	var b strings.Builder
	if first > 0 {
		b.WriteString(intPart[:first])
	}
	for i := first; i < n; i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(intPart[i : i+3])
	}
	return b.String()
}
