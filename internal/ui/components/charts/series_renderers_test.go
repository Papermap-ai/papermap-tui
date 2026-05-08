package charts

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// timeSeriesTable returns a 6-point monthly series with rising values, a
// shape that stresses the braille canvas projection without producing a
// degenerate constant or single-point series.
func timeSeriesTable() *api.InsightTable {
	return &api.InsightTable{
		Columns: []string{"month", "revenue"},
		Rows: [][]string{
			{"Jan", "100"},
			{"Feb", "150"},
			{"Mar", "120"},
			{"Apr", "200"},
			{"May", "180"},
			{"Jun", "240"},
		},
	}
}

func TestLine_RendersBrailleAndAxis(t *testing.T) {
	t.Parallel()

	tbl := timeSeriesTable()
	cfg := api.ChartConfig{Title: "Revenue", LabelKey: "month", YKey: "revenue"}
	out := Line(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 14})
	plain := stripANSI(out)

	if !strings.Contains(plain, "Revenue") {
		t.Fatalf("expected title in output:\n%s", plain)
	}
	if !strings.Contains(plain, "─") {
		t.Fatalf("expected bottom-rule chrome:\n%s", plain)
	}
	// At least one braille glyph should appear (any rune in U+2801..U+28FF).
	if !containsBraille(plain) {
		t.Fatalf("expected braille glyph in output:\n%s", plain)
	}
	// y-axis ticks should surface the max value (240) in some abbreviated form.
	if !strings.Contains(plain, "240") {
		t.Fatalf("expected max y tick label 240:\n%s", plain)
	}
	// x-axis tick labels should pull from the month column.
	if !strings.Contains(plain, "Jan") || !strings.Contains(plain, "Jun") {
		t.Fatalf("expected first/last x-axis labels:\n%s", plain)
	}
}

func TestLine_NoNumericData(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"k", "v"},
		Rows:    [][]string{{"a", "x"}},
	}
	out := Line(DefaultPalette(), tbl, api.ChartConfig{}, Size{Width: 60, Height: 12})
	if !strings.Contains(stripANSI(out), "unavailable") {
		t.Fatalf("expected unavailable notice, got:\n%s", out)
	}
}

func TestLine_TooSmall(t *testing.T) {
	t.Parallel()

	tbl := timeSeriesTable()
	cfg := api.ChartConfig{LabelKey: "month", YKey: "revenue"}

	out := Line(DefaultPalette(), tbl, cfg, Size{Width: 12, Height: 12})
	if !strings.Contains(stripANSI(out), "unavailable") {
		t.Fatalf("expected width-unavailable notice, got:\n%s", out)
	}

	out = Line(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 4})
	if !strings.Contains(stripANSI(out), "unavailable") {
		t.Fatalf("expected height-unavailable notice, got:\n%s", out)
	}
}

func TestArea_FillsBelowCurve(t *testing.T) {
	t.Parallel()

	tbl := timeSeriesTable()
	cfg := api.ChartConfig{Title: "Cumulative", LabelKey: "month", YKey: "revenue"}
	line := stripANSI(Line(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 14}))
	area := stripANSI(Area(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 14}))

	if !containsBraille(area) {
		t.Fatalf("expected braille glyphs in area output:\n%s", area)
	}
	// Area should set strictly more dots than line for the same series
	// because every column is filled down to the baseline. We approximate
	// "more dots" by counting non-blank braille runes.
	if countBraille(area) <= countBraille(line) {
		t.Fatalf("area should fill more cells than line\nline:\n%s\narea:\n%s", line, area)
	}
}

func TestScatter_NumericXAxis(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"hours", "score"},
		Rows: [][]string{
			{"1", "10"},
			{"3", "30"},
			{"5", "20"},
			{"7", "50"},
			{"9", "40"},
		},
	}
	cfg := api.ChartConfig{XKey: "hours", YKey: "score", Title: "Hours vs Score"}
	out := Scatter(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 14})
	plain := stripANSI(out)

	if !strings.Contains(plain, "Hours vs Score") {
		t.Fatalf("expected title:\n%s", plain)
	}
	if !containsBraille(plain) {
		t.Fatalf("expected braille glyphs:\n%s", plain)
	}
	if strings.Contains(plain, "rendered as bar") {
		t.Fatalf("scatter with numeric x must not fall back to bar:\n%s", plain)
	}
}

func TestScatter_NonNumericXAxisFallsBackToBar(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"category", "value"},
		Rows: [][]string{
			{"alpha", "10"},
			{"beta", "20"},
			{"gamma", "15"},
		},
	}
	cfg := api.ChartConfig{XKey: "category", YKey: "value", LabelKey: "category"}
	out := Scatter(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 12})
	plain := stripANSI(out)

	if !strings.Contains(plain, "rendered as bar") {
		t.Fatalf("expected bar-fallback note:\n%s", plain)
	}
	if !strings.Contains(plain, "█") {
		t.Fatalf("expected bar glyph in fallback:\n%s", plain)
	}
}

func TestScatter_MissingXKeyFallsBackToBar(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"label", "value"},
		Rows: [][]string{
			{"a", "10"},
			{"b", "20"},
		},
	}
	cfg := api.ChartConfig{LabelKey: "label", YKey: "value"}
	out := Scatter(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 12})
	if !strings.Contains(stripANSI(out), "rendered as bar") {
		t.Fatalf("expected bar fallback note when XKey absent:\n%s", out)
	}
}

func TestRadar_RendersAsBarWithHint(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"axis", "score"},
		Rows: [][]string{
			{"speed", "70"},
			{"accuracy", "85"},
			{"recall", "60"},
			{"precision", "90"},
		},
	}
	cfg := api.ChartConfig{LabelKey: "axis", YKey: "score"}
	out := Radar(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 12})
	plain := stripANSI(out)

	for _, want := range []string{"speed", "accuracy", "recall", "precision", "█"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in radar output:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "radar: per-axis values") {
		t.Fatalf("expected radar hint when no subtitle set:\n%s", plain)
	}
}

func TestRadar_OmitsHintWhenSubtitlePresent(t *testing.T) {
	t.Parallel()

	tbl := &api.InsightTable{
		Columns: []string{"axis", "score"},
		Rows:    [][]string{{"a", "1"}, {"b", "2"}},
	}
	cfg := api.ChartConfig{
		LabelKey: "axis", YKey: "score",
		Subtitle: "Q1 model evaluation",
	}
	out := stripANSI(Radar(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 12}))
	if strings.Contains(out, "radar: per-axis values") {
		t.Fatalf("hint should be omitted when subtitle present:\n%s", out)
	}
	if !strings.Contains(out, "Q1 model evaluation") {
		t.Fatalf("expected subtitle in output:\n%s", out)
	}
}

// containsBraille reports whether s contains any rune in the Braille
// pattern block, used as a structural assertion that the canvas drew
// at least one mark.
func containsBraille(s string) bool {
	for _, r := range s {
		if r >= 0x2801 && r <= 0x28FF {
			return true
		}
	}
	return false
}

func countBraille(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x2801 && r <= 0x28FF {
			n++
		}
	}
	return n
}
