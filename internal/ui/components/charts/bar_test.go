package charts

import (
	"regexp"
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// ansiRE strips terminal escapes so golden assertions stay deterministic
// across color schemes and terminal capabilities.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func tableFor(rows ...[]string) *api.InsightTable {
	return &api.InsightTable{
		Columns: []string{"name", "value"},
		Rows:    rows,
	}
}

func TestBar_RendersHorizontalBars(t *testing.T) {
	t.Parallel()
	tbl := tableFor(
		[]string{"alpha", "10"},
		[]string{"beta", "20"},
		[]string{"gamma", "30"},
	)
	cfg := api.ChartConfig{Title: "Sales", LabelKey: "name", YKey: "value"}
	out := Bar(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 10})
	plain := stripANSI(out)

	if !strings.Contains(plain, "Sales") {
		t.Fatalf("expected title in output:\n%s", plain)
	}
	for _, want := range []string{"alpha", "beta", "gamma", "10", "20", "30"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in output:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "█") {
		t.Fatalf("expected bar glyph in output:\n%s", plain)
	}
	// gamma (30) should produce more bar cells than alpha (10).
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	var alphaCount, gammaCount int
	for _, line := range lines {
		switch {
		case strings.Contains(line, "alpha"):
			alphaCount = strings.Count(line, "█")
		case strings.Contains(line, "gamma"):
			gammaCount = strings.Count(line, "█")
		}
	}
	if gammaCount <= alphaCount {
		t.Fatalf("expected gamma bar wider than alpha: alpha=%d gamma=%d\n%s",
			alphaCount, gammaCount, plain)
	}
}

func TestBar_NoNumericData(t *testing.T) {
	t.Parallel()
	tbl := &api.InsightTable{
		Columns: []string{"name", "value"},
		Rows:    [][]string{{"a", "x"}, {"b", "y"}},
	}
	out := Bar(DefaultPalette(), tbl, api.ChartConfig{}, Size{Width: 40, Height: 6})
	if !strings.Contains(stripANSI(out), "unavailable") {
		t.Fatalf("expected unavailable notice, got:\n%s", out)
	}
}

func TestBar_TooNarrow(t *testing.T) {
	t.Parallel()
	tbl := tableFor(
		[]string{"a-long-category-label", "1000"},
		[]string{"another-long-label", "2000"},
	)
	out := Bar(DefaultPalette(), tbl,
		api.ChartConfig{LabelKey: "name", YKey: "value"},
		Size{Width: 9, Height: 6})
	plain := stripANSI(out)
	if !strings.Contains(plain, "unavailable") {
		t.Fatalf("expected unavailable notice for narrow size, got:\n%s", plain)
	}
}

func TestBar_TruncatesAndReportsHidden(t *testing.T) {
	t.Parallel()
	rows := make([][]string, 0, 6)
	for i := 0; i < 6; i++ {
		rows = append(rows, []string{string(rune('a' + i)), "10"})
	}
	tbl := tableFor(rows...)
	cfg := api.ChartConfig{LabelKey: "name", YKey: "value"}
	// Height 4 means: 0 header lines + ~3 plot lines + 1 footer.
	out := Bar(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 4})
	plain := stripANSI(out)
	if !strings.Contains(plain, "hidden") {
		t.Fatalf("expected hidden footer, got:\n%s", plain)
	}
}

func TestBar_HandlesNegativeValues(t *testing.T) {
	t.Parallel()
	tbl := tableFor(
		[]string{"loss", "-15"},
		[]string{"gain", "20"},
	)
	cfg := api.ChartConfig{LabelKey: "name", YKey: "value"}
	out := Bar(DefaultPalette(), tbl, cfg, Size{Width: 60, Height: 6})
	plain := stripANSI(out)
	if !strings.Contains(plain, "░") {
		t.Fatalf("expected light-shade glyph for negative value:\n%s", plain)
	}
	if !strings.Contains(plain, "-15") {
		t.Fatalf("expected negative value label:\n%s", plain)
	}
}
