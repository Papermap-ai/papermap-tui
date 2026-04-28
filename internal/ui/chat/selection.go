package chat

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// selection tracks an active mouse-drag highlight over the transcript
// viewport. Anchor is the cell the user pressed at; cursor is the cell
// the mouse is currently over (or where it last moved). Coordinates are
// in transcript content space: line is a 0-based index into the lines
// of the rendered (ANSI-stripped) transcript; column is a 0-based
// rune-cell index within that line, clamped to the line's display
// width. A selection with anchor == cursor highlights nothing and is
// treated as a click.
type selection struct {
	active                bool
	anchorLine, anchorCol int
	cursorLine, cursorCol int
	// transcript caches the most recent ANSI-stripped content used to
	// compute byte ranges. Mirrored from syncViewportContent so the
	// extraction path doesn't have to re-strip on every drag tick.
	transcript string
}

// reset clears the selection without dropping the transcript cache.
// Called on release (after copying), on escape, and whenever the
// transcript is rebuilt under a live drag.
func (s *selection) reset() {
	s.active = false
	s.anchorLine, s.anchorCol = 0, 0
	s.cursorLine, s.cursorCol = 0, 0
}

// rememberTranscript stores the stripped form of the rendered
// transcript so byteRange and selectedText work in O(content) instead
// of O(content) per drag tick.
func (s *selection) rememberTranscript(stripped string) {
	s.transcript = stripped
}

// orderedBounds returns the (line, col) pair in document order, so
// callers don't have to care which direction the user dragged.
func (s selection) orderedBounds() (startLine, startCol, endLine, endCol int) {
	if s.anchorLine < s.cursorLine ||
		(s.anchorLine == s.cursorLine && s.anchorCol <= s.cursorCol) {
		return s.anchorLine, s.anchorCol, s.cursorLine, s.cursorCol
	}
	return s.cursorLine, s.cursorCol, s.anchorLine, s.anchorCol
}

// isEmpty reports whether the anchor and cursor are at the same cell.
// Empty selections produce no highlight and no clipboard write.
func (s selection) isEmpty() bool {
	return s.anchorLine == s.cursorLine && s.anchorCol == s.cursorCol
}

// byteRange computes the [start, end) byte offsets into the cached
// transcript for the current selection. Returns ok=false when the
// selection is empty or the transcript is unavailable. Used by
// selectedText to slice the clipboard payload; not used for rendering
// the on-screen highlight (see applyHighlight in highlight.go).
func (s selection) byteRange() (start, end int, ok bool) {
	if s.transcript == "" || s.isEmpty() {
		return 0, 0, false
	}
	startLine, startCol, endLine, endCol := s.orderedBounds()
	startByte, sok := lineColToByte(s.transcript, startLine, startCol)
	endByte, eok := lineColToByte(s.transcript, endLine, endCol)
	if !sok || !eok || endByte <= startByte {
		return 0, 0, false
	}
	return startByte, endByte, true
}

// selectedText returns the substring of the cached transcript covered
// by the selection, normalized to LF line endings. Empty selections
// return "". Callers should not invoke this while the transcript is
// being rebuilt.
//
// TODO: lineColToByte treats col as a rune index but orderedBounds
// returns grapheme-cell columns. For ASCII content these agree; for
// wide characters (CJK, emoji) the clipboard slice will be off by
// the cell-vs-rune delta. Fix by walking the same uv.Buffer cells
// applyHighlight uses and joining cell.Content directly.
func (s selection) selectedText() string {
	start, end, ok := s.byteRange()
	if !ok {
		return ""
	}
	if end > len(s.transcript) {
		end = len(s.transcript)
	}
	return s.transcript[start:end]
}

// lineColToByte converts a (line, col) cell coordinate in the stripped
// transcript into a byte offset. line is 0-based; col is the rune index
// within the line, clamped to the line's rune length so out-of-range
// drags select to end-of-line instead of bleeding into the next line.
// Returns ok=false when line is past the document.
func lineColToByte(content string, line, col int) (int, bool) {
	if line < 0 || col < 0 {
		return 0, false
	}
	pos := 0
	currentLine := 0
	for currentLine < line {
		nl := strings.IndexByte(content[pos:], '\n')
		if nl < 0 {
			return 0, false
		}
		pos += nl + 1
		currentLine++
	}
	// pos now sits at the start of the target line.
	lineEnd := strings.IndexByte(content[pos:], '\n')
	var lineSlice string
	if lineEnd < 0 {
		lineSlice = content[pos:]
	} else {
		lineSlice = content[pos : pos+lineEnd]
	}
	// Walk col runes (or fewer, if the line is shorter).
	colsLeft := col
	for colsLeft > 0 {
		if lineSlice == "" {
			break
		}
		_, size := utf8.DecodeRuneInString(lineSlice)
		if size == 0 {
			break
		}
		lineSlice = lineSlice[size:]
		pos += size
		colsLeft--
	}
	return pos, true
}

// stripContent returns the transcript with all ANSI escape sequences
// removed. Used both for the rune-cell mapping and to produce the
// clipboard payload.
func stripContent(rendered string) string {
	return ansi.Strip(rendered)
}
