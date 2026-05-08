package charts

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// brailleCanvas is a virtual dot grid backed by Unicode Braille cells.
// Each terminal cell holds a 2x4 grid of dots, so a canvas of (cols, rows)
// terminal cells exposes a (cols*2, rows*4) dot space. The line, area,
// and scatter renderers all draw into one of these so axis chrome,
// projection math, and color handling stay in one place.
//
// Coordinates passed to setDot are dot coordinates, not terminal cells:
//
//	(0, 0) is the top-left dot;
//	x in [0, cols*2)  grows rightward;
//	y in [0, rows*4)  grows downward.
//
// Out-of-range writes are silently ignored — projection helpers may emit
// rounded values that drift one dot past the edge and a no-op is the
// honest behavior in a fixed-size canvas.
type brailleCanvas struct {
	cells [][]uint8
	cols  int
	rows  int
}

// brailleDotMask maps (xInCell, yInCell) to the bit it occupies inside a
// Braille pattern byte. The Unicode layout is column-major:
//
//	col 0      col 1
//	0x01       0x08
//	0x02       0x10
//	0x04       0x20
//	0x40       0x80
var brailleDotMask = [2][4]uint8{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// newBrailleCanvas allocates a canvas sized to (cols, rows) terminal
// cells. cols or rows < 1 produce an unusable canvas whose render method
// returns "" — callers should guard with a "not enough space" check
// before allocating.
func newBrailleCanvas(cols, rows int) *brailleCanvas {
	if cols < 1 || rows < 1 {
		return &brailleCanvas{}
	}
	cells := make([][]uint8, rows)
	for i := range cells {
		cells[i] = make([]uint8, cols)
	}
	return &brailleCanvas{cells: cells, cols: cols, rows: rows}
}

// setDot turns on the braille dot at (x, y) in dot coordinates. Writes
// outside the canvas are dropped.
func (c *brailleCanvas) setDot(x, y int) {
	if c.rows == 0 || x < 0 || y < 0 {
		return
	}
	col := x / 2
	row := y / 4
	if col >= c.cols || row >= c.rows {
		return
	}
	c.cells[row][col] |= brailleDotMask[x%2][y%4]
}

// fillColumn turns on every dot from yTop down through yBot (inclusive)
// at dot column x. Used by area to fill below the curve.
func (c *brailleCanvas) fillColumn(x, yTop, yBot int) {
	if yTop > yBot {
		yTop, yBot = yBot, yTop
	}
	for y := yTop; y <= yBot; y++ {
		c.setDot(x, y)
	}
}

// render emits the canvas as a string of braille runes, one row per line.
// Empty cells render as the blank braille pattern (U+2800) so column
// alignment is preserved when the canvas is composed next to axis labels.
// fg is applied to every non-empty cell; the blank cell stays unstyled
// so it does not waste escape bytes when no dot is set.
func (c *brailleCanvas) render(fg color.Color) string {
	if c.rows == 0 {
		return ""
	}
	const base rune = 0x2800
	style := lipgloss.NewStyle().Foreground(fg)
	lines := make([]string, c.rows)
	for r, row := range c.cells {
		var b strings.Builder
		b.Grow(c.cols * 4)
		for _, mask := range row {
			ch := string(base + rune(mask))
			if mask == 0 {
				b.WriteString(" ")
				continue
			}
			b.WriteString(style.Render(ch))
		}
		lines[r] = b.String()
	}
	return strings.Join(lines, "\n")
}

// xyBounds returns the min/max for x and y across pts. Empty input yields
// zeros and ok=false. Constant series (min == max) collapse to a single
// horizontal line; callers handle the degenerate case by widening the
// range so projection math does not divide by zero.
func xyBounds(pts []Point) (xMin, xMax, yMin, yMax float64, ok bool) {
	if len(pts) == 0 {
		return 0, 0, 0, 0, false
	}
	xMin, xMax = pts[0].X, pts[0].X
	yMin, yMax = pts[0].Y, pts[0].Y
	for _, p := range pts[1:] {
		switch {
		case p.X < xMin:
			xMin = p.X
		case p.X > xMax:
			xMax = p.X
		}
		switch {
		case p.Y < yMin:
			yMin = p.Y
		case p.Y > yMax:
			yMax = p.Y
		}
	}
	return xMin, xMax, yMin, yMax, true
}

// project maps a value v from [lo, hi] into the integer range [0, span-1].
// When lo == hi the mapping collapses to the midpoint so the rendered
// mark sits in the middle of the available space rather than glued to an
// edge. span < 1 returns 0.
func project(v, lo, hi float64, span int) int {
	if span < 1 {
		return 0
	}
	if hi == lo {
		return span / 2
	}
	r := (v - lo) / (hi - lo)
	if r < 0 {
		r = 0
	} else if r > 1 {
		r = 1
	}
	out := int(r * float64(span-1))
	if out < 0 {
		out = 0
	} else if out >= span {
		out = span - 1
	}
	return out
}

// axisYTickWidth is the column width reserved for y-axis tick labels.
// formatAxisValue caps at 5 chars (e.g. "1.5k"); add 1 for a left margin
// from the canvas border so labels don't run flush against the bars.
const axisYTickWidth = 6

// axisFrame composes a y-tick column, the rendered braille canvas, a
// bottom rule, and 3 x-axis tick labels into a single string. The result
// fits inside a (size.Width, size.Height - headerLines) box. headerLines
// is honored by the caller; this function only draws the chart body.
//
// yMin/yMax/xMin/xMax describe the data range driving the canvas. Tick
// labels use formatAxisValue so they stay narrow at any magnitude.
//
// The xLabels slice supplies categorical labels for the x axis (used by
// line/area where x is a row index but the original column is a date or
// category). When xLabels is nil or shorter than 2, numeric tick labels
// are derived from xMin/xMax instead.
func axisFrame(canvas *brailleCanvas, p Palette, fg color.Color, yMin, yMax, xMin, xMax float64, xLabels []string) string {
	if canvas == nil || canvas.rows == 0 {
		return ""
	}

	body := canvas.render(fg)
	bodyLines := strings.Split(body, "\n")

	// Y tick labels at top, middle, bottom of the canvas. Renderers ask
	// for canvases with at least 3 rows so the three labels never collide.
	yLabel := lipgloss.NewStyle().Foreground(p.Axis).Width(axisYTickWidth - 1).Align(lipgloss.Right)
	yTicks := make([]string, canvas.rows)
	for i := range yTicks {
		switch i {
		case 0:
			yTicks[i] = yLabel.Render(formatAxisValue(yMax)) + " "
		case canvas.rows / 2:
			yTicks[i] = yLabel.Render(formatAxisValue((yMin+yMax)/2)) + " "
		case canvas.rows - 1:
			yTicks[i] = yLabel.Render(formatAxisValue(yMin)) + " "
		default:
			yTicks[i] = strings.Repeat(" ", axisYTickWidth)
		}
	}

	rows := make([]string, canvas.rows)
	for i := range rows {
		line := ""
		if i < len(bodyLines) {
			line = bodyLines[i]
		}
		rows[i] = yTicks[i] + line
	}

	axisStyle := lipgloss.NewStyle().Foreground(p.Axis)
	rule := strings.Repeat(" ", axisYTickWidth) + axisStyle.Render(strings.Repeat("─", canvas.cols))

	xTickLine := xAxisTicks(p, canvas.cols, xMin, xMax, xLabels)
	if xTickLine != "" {
		xTickLine = strings.Repeat(" ", axisYTickWidth) + xTickLine
	}

	parts := make([]string, 0, len(rows)+2)
	parts = append(parts, rows...)
	parts = append(parts, rule)
	if xTickLine != "" {
		parts = append(parts, xTickLine)
	}
	return strings.Join(parts, "\n")
}

// xAxisTicks renders the x-axis tick label row. Three positions are
// labeled when there is room: left (xMin / xLabels[0]), middle, right
// (xMax / xLabels[last]). Returns "" when the canvas is too narrow for
// even a left+right pair.
func xAxisTicks(p Palette, cols int, xMin, xMax float64, xLabels []string) string {
	if cols < 4 {
		return ""
	}
	style := lipgloss.NewStyle().Foreground(p.Axis)

	left, mid, right := "", "", ""
	if len(xLabels) >= 2 {
		left = xLabels[0]
		right = xLabels[len(xLabels)-1]
		if len(xLabels) >= 3 {
			mid = xLabels[len(xLabels)/2]
		}
	} else {
		left = formatAxisValue(xMin)
		right = formatAxisValue(xMax)
		if xMax != xMin {
			mid = formatAxisValue((xMin + xMax) / 2)
		}
	}

	// Truncate each label to a third of the canvas to stop long category
	// names from overlapping each other on narrow charts.
	maxTickWidth := cols / 3
	if maxTickWidth < 3 {
		maxTickWidth = 3
	}
	left = truncate(left, maxTickWidth)
	mid = truncate(mid, maxTickWidth)
	right = truncate(right, maxTickWidth)

	leftW := lipgloss.Width(left)
	midW := lipgloss.Width(mid)
	rightW := lipgloss.Width(right)

	// Layout: left at column 0, right flush to the end of the axis,
	// middle centered on the canvas midpoint when it fits without
	// touching either neighbor.
	out := make([]rune, cols)
	for i := range out {
		out[i] = ' '
	}
	writeAt(out, 0, left)

	if midW > 0 {
		midStart := (cols - midW) / 2
		if midStart > leftW && midStart+midW < cols-rightW {
			writeAt(out, midStart, mid)
		}
	}

	if rightW > 0 {
		rightStart := cols - rightW
		if rightStart > leftW {
			writeAt(out, rightStart, right)
		}
	}

	return style.Render(string(out))
}

// writeAt copies src runes into dst starting at offset. Out-of-range
// writes are clipped to dst. dst is mutated in place.
func writeAt(dst []rune, offset int, src string) {
	for i, r := range src {
		pos := offset + i
		if pos < 0 || pos >= len(dst) {
			return
		}
		dst[pos] = r
	}
}
