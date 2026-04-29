package chat

import (
	"strings"
	"testing"
)

// TestLineColToByteFirstLine verifies the basic mapping for offsets
// inside the first line of the document.
func TestLineColToByteFirstLine(t *testing.T) {
	t.Parallel()

	content := "hello world\nsecond line\n"
	got, ok := lineColToByte(content, 0, 6)
	if !ok {
		t.Fatal("expected ok=true for valid offset")
	}
	if got != 6 {
		t.Fatalf("expected byte offset 6, got %d", got)
	}
}

// TestLineColToByteSecondLine verifies the offset accounts for the
// preceding newline byte when targeting a later line.
func TestLineColToByteSecondLine(t *testing.T) {
	t.Parallel()

	content := "hello world\nsecond line\n"
	got, ok := lineColToByte(content, 1, 7)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// "hello world\n" = 12 bytes, plus 7 cols into "second line" = 19.
	if got != 19 {
		t.Fatalf("expected byte offset 19, got %d", got)
	}
}

// TestLineColToByteClampsPastLineEnd verifies that a column past the
// end of the line clamps to the line end (the newline byte) instead
// of bleeding into the next line. This matters for drag selection: a
// user dragging far to the right of a short line should only highlight
// up to the line's last cell, not wrap onto the next line.
func TestLineColToByteClampsPastLineEnd(t *testing.T) {
	t.Parallel()

	content := "hi\nlonger line\n"
	got, ok := lineColToByte(content, 0, 50)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// "hi" is 2 bytes; col 50 should clamp to position right at the
	// newline (byte 2), not advance into the next line.
	if got != 2 {
		t.Fatalf("expected byte offset 2 (clamped to end of 'hi'), got %d", got)
	}
}

// TestLineColToBytePastDocument verifies the function returns ok=false
// when the line index is past the end of the document so callers can
// reject out-of-range drags rather than silently selecting nothing.
func TestLineColToBytePastDocument(t *testing.T) {
	t.Parallel()

	content := "only one line"
	if _, ok := lineColToByte(content, 5, 0); ok {
		t.Fatal("expected ok=false for line past document end")
	}
}

// TestSelectionByteRangeOrderInvariant verifies the byte range is
// computed in document order regardless of which direction the user
// dragged. Backward drags (cursor before anchor) should still produce
// a forward [start, end) range.
func TestSelectionByteRangeOrderInvariant(t *testing.T) {
	t.Parallel()

	content := "aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc\n"

	forward := selection{
		active:     true,
		anchorLine: 0, anchorCol: 2,
		cursorLine: 2, cursorCol: 5,
		transcript: content,
	}
	fStart, fEnd, fOK := forward.byteRange()
	if !fOK {
		t.Fatal("forward selection should produce a range")
	}

	backward := selection{
		active:     true,
		anchorLine: 2, anchorCol: 5,
		cursorLine: 0, cursorCol: 2,
		transcript: content,
	}
	bStart, bEnd, bOK := backward.byteRange()
	if !bOK {
		t.Fatal("backward selection should produce a range")
	}

	if fStart != bStart || fEnd != bEnd {
		t.Fatalf("forward and backward ranges should match: forward=[%d,%d) backward=[%d,%d)",
			fStart, fEnd, bStart, bEnd)
	}
}

// TestSelectionEmptyProducesNoRange verifies that anchor==cursor
// produces no clipboard payload and no highlight. Without this guard,
// every click would copy a zero-width range.
func TestSelectionEmptyProducesNoRange(t *testing.T) {
	t.Parallel()

	s := selection{
		active:     true,
		anchorLine: 1, anchorCol: 4,
		cursorLine: 1, cursorCol: 4,
		transcript: "line one\nline two\n",
	}
	if _, _, ok := s.byteRange(); ok {
		t.Fatal("empty selection should not produce a byte range")
	}
	if got := s.selectedText(); got != "" {
		t.Fatalf("empty selection should yield empty text, got %q", got)
	}
}

// TestSelectionTextRoundtrip verifies the extracted substring matches
// what the user visually highlighted across multiple lines.
func TestSelectionTextRoundtrip(t *testing.T) {
	t.Parallel()

	content := "the quick brown\nfox jumps over\nthe lazy dog\n"
	s := selection{
		active:     true,
		anchorLine: 0, anchorCol: 4,
		cursorLine: 1, cursorCol: 9,
		transcript: content,
	}
	got := s.selectedText()
	want := "quick brown\nfox jumps"
	if got != want {
		t.Fatalf("selected text mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestStripContentRemovesAnsi verifies the stripper used to feed the
// selection cache actually removes SGR sequences. Without stripping,
// rune-cell math would be wildly off.
func TestStripContentRemovesAnsi(t *testing.T) {
	t.Parallel()

	rendered := "\x1b[1;31mhello\x1b[0m world"
	stripped := stripContent(rendered)
	if strings.Contains(stripped, "\x1b") {
		t.Fatalf("stripContent should remove escape bytes, got %q", stripped)
	}
	if stripped != "hello world" {
		t.Fatalf("expected 'hello world', got %q", stripped)
	}
}
