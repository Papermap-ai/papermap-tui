package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// applyHighlight returns content with the cells inside the
// half-open rectangle (startLine, startCol) -> (endLine, endCol)
// rendered with style replacing each affected cell's existing style.
//
// Coordinates are zero-based grapheme-cell positions in the rendered
// (already-styled) transcript. The caller is responsible for ordering
// bounds (selection.orderedBounds) and clamping endCol against the
// trailing-whitespace logic below.
//
// We pre-render rather than relying on the bubbles viewport
// SetHighlights pipeline because that pipeline emits empty styled
// spans for zero-width grapheme cuts on certain content shapes,
// leaving the highlight visually invisible. Replacing cell styles
// on a uv.ScreenBuffer guarantees the final SGR sequence is applied
// to actual content cells.
//
// width and height define the buffer the transcript is drawn into,
// matching the viewport's display dimensions.
//
// Empty range (start == end) returns content unchanged.
func applyHighlight(
	content string,
	width, height int,
	startLine, startCol, endLine, endCol int,
	style lipgloss.Style,
) string {
	if width <= 0 || height <= 0 {
		return content
	}
	if startLine == endLine && startCol == endCol {
		return content
	}
	if startLine < 0 || startCol < 0 {
		return content
	}

	hi := styleToUV(style)

	buf := uv.NewScreenBuffer(width, height)
	area := uv.Rect(0, 0, width, height)
	styled := uv.NewStyledString(content)
	styled.Draw(buf, area)

	for y := startLine; y <= endLine && y < buf.Height(); y++ {
		line := buf.Line(y)
		colsInLine := len(line)

		colStart := 0
		if y == startLine {
			colStart = clampInt(startCol, 0, colsInLine)
		}
		colEnd := colsInLine
		if y == endLine {
			colEnd = clampInt(endCol, colStart, colsInLine)
		}

		// Reverse-scan to find the last cell carrying real content so
		// we stop the highlight there. This avoids painting trailing
		// padding spaces and gives the band a swept-highlighter feel
		// rather than extending into empty terminal space.
		highlightEnd := colStart
		for x := colEnd - 1; x >= colStart; x-- {
			cell := line.At(x)
			if cell == nil {
				continue
			}
			if cell.Content != "" && cell.Content != " " {
				highlightEnd = x + 1
				break
			}
		}

		for x := colStart; x < highlightEnd; x++ {
			cell := line.At(x)
			if cell == nil {
				continue
			}
			cell.Style = hi
		}
	}

	// Buffer.Render pads every line to width with spaces and emits
	// height lines. Trim trailing all-blank lines so embedding the
	// result back in the viewport does not inflate the row count. We
	// intentionally do not strip per-line trailing whitespace because
	// styled cells (e.g. code-fence backgrounds) may render as space
	// glyphs that carry meaningful background color.
	return trimTrailingBlankLines(buf.Render())
}

// styleToUV projects a lipgloss.Style into the subset of uv.Style we
// care about for selection painting (fg, bg, bold/italic/reverse,
// underline). Underline is forwarded so that selecting over a link
// preserves its underline under the highlight bg.
func styleToUV(s lipgloss.Style) uv.Style {
	var out uv.Style
	out.Fg = s.GetForeground()
	out.Bg = s.GetBackground()
	var attrs uint8
	if s.GetBold() {
		attrs |= uv.AttrBold
	}
	if s.GetItalic() {
		attrs |= uv.AttrItalic
	}
	if s.GetReverse() {
		attrs |= uv.AttrReverse
	}
	out.Attrs = attrs
	if s.GetUnderline() {
		out.Underline = uv.UnderlineSingle
	}
	return out
}

// trimTrailingBlankLines drops trailing lines whose content is only
// whitespace so the rendered buffer height matches the actual content
// height when handed back to the viewport.
func trimTrailingBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
