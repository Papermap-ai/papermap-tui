package charts

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is the minimal styling surface every renderer needs. Decoupling
// charts from theme.Theme keeps the package independently testable and
// makes the contract explicit: charts care about series colors, axis and
// grid foregrounds, label and muted text.
type Palette struct {
	// Series is the rotation of foreground colors used for chart marks
	// (bars, lines, dots). Renderers index into Series modulo len.
	// Must contain at least one entry; zero-len palettes fall back to
	// DefaultPalette.
	Series []color.Color
	// Axis is the color used for axis lines and tick marks.
	Axis color.Color
	// Grid is the color for any background grid (lighter than Axis).
	Grid color.Color
	// Label is the color used for tick labels and series legends.
	Label color.Color
	// Muted is used for empty-state text and the unavailable-chart
	// notice.
	Muted lipgloss.Style
}

// DefaultPalette returns a usable palette so renderers and their tests
// never have to construct one by hand. The colors are intentionally
// terminal-256 ANSI numbers to render predictably across themes.
func DefaultPalette() Palette {
	return Palette{
		Series: []color.Color{
			lipgloss.Color("42"),  // green
			lipgloss.Color("39"),  // blue
			lipgloss.Color("214"), // orange
			lipgloss.Color("213"), // pink
			lipgloss.Color("226"), // yellow
			lipgloss.Color("105"), // purple
		},
		Axis:  lipgloss.Color("245"),
		Grid:  lipgloss.Color("238"),
		Label: lipgloss.Color("252"),
		Muted: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// SeriesColor returns Series[index modulo len], falling back to the first
// DefaultPalette color when Series is empty.
func (p Palette) SeriesColor(index int) color.Color {
	if len(p.Series) == 0 {
		return DefaultPalette().Series[0]
	}
	if index < 0 {
		index = -index
	}
	return p.Series[index%len(p.Series)]
}
