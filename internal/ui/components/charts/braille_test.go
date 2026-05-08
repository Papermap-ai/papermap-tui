package charts

import (
	"strings"
	"testing"
)

func TestBrailleCanvas_SetDot(t *testing.T) {
	t.Parallel()

	c := newBrailleCanvas(4, 2) // 8 dots wide, 8 dots tall
	c.setDot(0, 0)
	c.setDot(7, 7)
	c.setDot(3, 4)

	out := stripANSI(c.render(DefaultPalette().SeriesColor(0)))
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 rows, got %d: %q", len(lines), out)
	}
	// Each row should be 4 chars wide. The blank cell is rendered as a
	// space when no dot is set, so width stays at 4.
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) != 4 {
			t.Fatalf("row %d width = %d (%q), want 4", i, len(runes), line)
		}
	}

	// Top-left cell must contain a non-blank braille character.
	if []rune(lines[0])[0] == ' ' {
		t.Fatalf("expected dot at top-left, got blank: %q", lines[0])
	}
	// Bottom-right cell likewise.
	if []rune(lines[1])[3] == ' ' {
		t.Fatalf("expected dot at bottom-right, got blank: %q", lines[1])
	}
}

func TestBrailleCanvas_OutOfRangeIsNoop(t *testing.T) {
	t.Parallel()

	c := newBrailleCanvas(2, 1)
	// All of these must be silently dropped.
	c.setDot(-1, 0)
	c.setDot(0, -1)
	c.setDot(100, 100)
	c.setDot(4, 0)
	c.setDot(0, 4)

	out := stripANSI(c.render(DefaultPalette().SeriesColor(0)))
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty canvas, got %q", out)
	}
}

func TestBrailleCanvas_FillColumn(t *testing.T) {
	t.Parallel()

	c := newBrailleCanvas(2, 2) // 4x8 dots
	c.fillColumn(0, 1, 6)

	// Both rows on dot column 0 should be non-blank.
	out := stripANSI(c.render(DefaultPalette().SeriesColor(0)))
	lines := strings.Split(out, "\n")
	if []rune(lines[0])[0] == ' ' || []rune(lines[1])[0] == ' ' {
		t.Fatalf("expected fill across both rows: %q", out)
	}
}

func TestProject_ConstantRange(t *testing.T) {
	t.Parallel()

	if got := project(5, 5, 5, 10); got != 5 {
		t.Fatalf("constant range: project(5,5,5,10) = %d, want 5 (midpoint)", got)
	}
}

func TestProject_Clamps(t *testing.T) {
	t.Parallel()

	if got := project(-100, 0, 10, 5); got != 0 {
		t.Fatalf("expected clamp to 0, got %d", got)
	}
	if got := project(1000, 0, 10, 5); got != 4 {
		t.Fatalf("expected clamp to span-1=4, got %d", got)
	}
}

func TestXyBounds_Empty(t *testing.T) {
	t.Parallel()

	if _, _, _, _, ok := xyBounds(nil); ok {
		t.Fatal("expected ok=false for empty input")
	}
}

func TestXyBounds_SinglePoint(t *testing.T) {
	t.Parallel()

	xMin, xMax, yMin, yMax, ok := xyBounds([]Point{{X: 3, Y: 7}})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if xMin != 3 || xMax != 3 || yMin != 7 || yMax != 7 {
		t.Fatalf("single-point bounds: got (%v,%v,%v,%v)", xMin, xMax, yMin, yMax)
	}
}
