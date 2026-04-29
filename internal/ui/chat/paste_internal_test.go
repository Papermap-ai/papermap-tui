package chat

import (
	"strings"
	"testing"
)

func TestShouldCollapsePaste(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"short single line", "hello", false},
		{"three lines", "a\nb\nc", false},
		{"four lines triggers", "a\nb\nc\nd", true},
		{"long single line triggers", strings.Repeat("x", pasteMinChars), true},
		{"just under char threshold", strings.Repeat("x", pasteMinChars-1), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldCollapsePaste(tc.in); got != tc.want {
				t.Fatalf("shouldCollapsePaste(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPasteRegistryAddAndExpand(t *testing.T) {
	t.Parallel()

	r := newPasteRegistry()
	tok1, lines1 := r.add("alpha\nbeta\ngamma\ndelta")
	if lines1 != 4 {
		t.Fatalf("expected 4 lines, got %d", lines1)
	}
	tok2, _ := r.add("second blob")

	buffer := "before " + tok1 + " middle " + tok2 + " end"
	expanded := r.expand(buffer)

	if strings.Contains(expanded, "[#p:") {
		t.Fatalf("expand left tokens behind: %q", expanded)
	}
	if !strings.Contains(expanded, "alpha\nbeta\ngamma\ndelta") {
		t.Fatalf("expand missing first chip: %q", expanded)
	}
	if !strings.Contains(expanded, "second blob") {
		t.Fatalf("expand missing second chip: %q", expanded)
	}
}

func TestPasteRegistryExpandLeavesUnknownTokenAlone(t *testing.T) {
	t.Parallel()

	r := newPasteRegistry()
	// No chips registered: a literal token should pass through untouched.
	got := r.expand("hello [#p:99] world")
	if got != "hello [#p:99] world" {
		t.Fatalf("unexpected expansion: %q", got)
	}
}

func TestChipBeforeCursorMatchesExactBoundary(t *testing.T) {
	t.Parallel()

	r := newPasteRegistry()
	tok, _ := r.add("blob")
	value := "prefix " + tok + " suffix"
	tokenStart := strings.Index(value, tok)
	tokenEnd := tokenStart + len(tok)

	chip, start, end, ok := r.chipBeforeCursor(value, tokenEnd)
	if !ok {
		t.Fatal("expected chipBeforeCursor to find chip at token end")
	}
	if start != tokenStart || end != tokenEnd {
		t.Fatalf("got range [%d,%d), want [%d,%d)", start, end, tokenStart, tokenEnd)
	}
	if chip.content != "blob" {
		t.Fatalf("unexpected chip content: %q", chip.content)
	}

	// Cursor one byte short of the token end should not match.
	if _, _, _, ok := r.chipBeforeCursor(value, tokenEnd-1); ok {
		t.Fatal("expected no match when cursor is mid-token")
	}
	// Cursor one byte past the token end should not match.
	if _, _, _, ok := r.chipBeforeCursor(value, tokenEnd+1); ok {
		t.Fatal("expected no match when cursor is past the token")
	}
}

func TestRuneIndexToByteOffset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		s    string
		idx  int
		want int
	}{
		{"abc", 0, 0},
		{"abc", 2, 2},
		{"abc", 3, 3},
		{"abc", 99, 3},
		{"héllo", 2, 3}, // 'é' is 2 bytes
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.s, func(t *testing.T) {
			t.Parallel()
			if got := runeIndexToByteOffset(tc.s, tc.idx); got != tc.want {
				t.Fatalf("runeIndexToByteOffset(%q, %d) = %d, want %d", tc.s, tc.idx, got, tc.want)
			}
		})
	}
}

func TestCursorByteOffsetMultiline(t *testing.T) {
	t.Parallel()

	value := "abc\ndef\nghi"
	// Line 1, col 2 = "de|f" -> byte offset 4 ('a','b','c','\n','d','e')
	if got := cursorByteOffset(value, 1, 2); got != 6 {
		t.Fatalf("cursorByteOffset = %d, want 6", got)
	}
	// Line 0, col 0
	if got := cursorByteOffset(value, 0, 0); got != 0 {
		t.Fatalf("cursorByteOffset = %d, want 0", got)
	}
	// Out of range line clamps to len(value).
	if got := cursorByteOffset(value, 99, 0); got != len(value) {
		t.Fatalf("cursorByteOffset out-of-range = %d, want %d", got, len(value))
	}
}
