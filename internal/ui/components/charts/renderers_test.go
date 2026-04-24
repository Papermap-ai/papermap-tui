package charts

import (
	"strconv"
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func growingTable(n int) *api.InsightTable {
	rows := make([][]string, n)
	for i := 0; i < n; i++ {
		rows[i] = []string{string(rune('a' + i)), strconv.Itoa(i + 1)}
	}
	return &api.InsightTable{
		Columns: []string{"name", "value"},
		Rows:    rows,
	}
}

func TestPie_RendersLegendAndPercents(t *testing.T) {
	t.Parallel()
	tbl := &api.InsightTable{
		Columns: []string{"category", "share"},
		Rows: [][]string{
			{"alpha", "50"}, {"beta", "30"}, {"gamma", "20"},
		},
	}
	out := Pie(DefaultPalette(), tbl,
		api.ChartConfig{LabelKey: "category", YKey: "share"},
		Size{Width: 50, Height: 8})
	plain := stripANSI(out)
	for _, want := range []string{"alpha", "beta", "gamma", "50%", "30%", "20%"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in pie output:\n%s", want, plain)
		}
	}
}

func TestPie_CollapsesToOther(t *testing.T) {
	t.Parallel()
	rows := [][]string{
		{"a", "100"},
		{"b", "50"},
		{"c", "40"},
		{"d", "30"},
		{"e", "20"},
		{"f", "10"},
		{"g", "5"},
		{"h", "5"},
	}
	tbl := &api.InsightTable{Columns: []string{"k", "v"}, Rows: rows}
	out := Pie(DefaultPalette(), tbl,
		api.ChartConfig{LabelKey: "k", YKey: "v"},
		Size{Width: 50, Height: 10})
	plain := stripANSI(out)
	if !strings.Contains(plain, "Other") {
		t.Fatalf("expected Other bucket:\n%s", plain)
	}
}

func TestPie_NoPositiveValues(t *testing.T) {
	t.Parallel()
	tbl := &api.InsightTable{
		Columns: []string{"k", "v"},
		Rows:    [][]string{{"a", "0"}, {"b", "-5"}},
	}
	out := Pie(DefaultPalette(), tbl,
		api.ChartConfig{LabelKey: "k", YKey: "v"},
		Size{Width: 40, Height: 6})
	if !strings.Contains(stripANSI(out), "unavailable") {
		t.Fatalf("expected unavailable notice:\n%s", out)
	}
}

func TestRender_DispatchesByType(t *testing.T) {
	t.Parallel()
	tbl := growingTable(4)
	cfg := api.ChartConfig{LabelKey: "name", YKey: "value"}
	for _, ct := range []string{"bar", "pie"} {
		out := Render(ct, DefaultPalette(), tbl, cfg, Size{Width: 50, Height: 8})
		if out == "" {
			t.Errorf("Render(%q) returned empty", ct)
		}
	}
	for _, ct := range []string{"line", "area", "scatter", "radar"} {
		if Render(ct, DefaultPalette(), tbl, cfg, Size{Width: 50, Height: 8}) != "" {
			t.Errorf("Render(%q) should not be registered", ct)
		}
	}
	if !IsSupported("BAR") {
		t.Errorf("IsSupported should be case-insensitive")
	}
	for _, ct := range []string{"line", "area", "scatter", "radar"} {
		if IsSupported(ct) {
			t.Errorf("IsSupported(%q) should return false", ct)
		}
	}
}
