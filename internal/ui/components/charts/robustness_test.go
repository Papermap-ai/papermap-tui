package charts

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// TestRender_PanicResistance feeds every registered renderer a battery of
// degenerate payloads and asserts that none of them panic. The point is
// not to assert a specific rendering — degenerate inputs may legitimately
// produce the muted "unavailable" notice or an empty string — only that
// the renderer does not crash. This protects against UTF-8 corruption,
// NaN/Inf escapes, and ragged-row indexing regressions.
func TestRender_PanicResistance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		table *api.InsightTable
		cfg   api.ChartConfig
	}{
		{
			name:  "nil table",
			table: nil,
		},
		{
			name:  "empty rows",
			table: &api.InsightTable{Columns: []string{"a", "b"}},
		},
		{
			name: "all non-numeric",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows:    [][]string{{"x", "alpha"}, {"y", "beta"}},
			},
		},
		{
			name: "ragged rows shorter than header",
			table: &api.InsightTable{
				Columns: []string{"k", "v", "extra"},
				Rows:    [][]string{{"a", "1"}, {"b"}},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
		{
			name: "infinity string",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows:    [][]string{{"a", "Inf"}, {"b", "-inf"}, {"c", "5"}},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
		{
			name: "nan string",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows:    [][]string{{"a", "NaN"}, {"b", "10"}},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
		{
			name: "single point",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows:    [][]string{{"only", "42"}},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
		{
			name: "constant series",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows:    [][]string{{"a", "5"}, {"b", "5"}, {"c", "5"}},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
		{
			name: "unicode labels",
			table: &api.InsightTable{
				Columns: []string{"city", "v"},
				Rows: [][]string{
					{"東京", "100"}, {"München", "80"}, {"São Paulo", "60"},
				},
			},
			cfg: api.ChartConfig{LabelKey: "city", YKey: "v"},
		},
		{
			name: "very long labels",
			table: &api.InsightTable{
				Columns: []string{"k", "v"},
				Rows: [][]string{
					{strings.Repeat("a", 200), "1"},
					{strings.Repeat("b", 200), "2"},
				},
			},
			cfg: api.ChartConfig{LabelKey: "k", YKey: "v"},
		},
	}

	chartTypes := []string{"bar", "pie", "line", "area", "scatter", "radar"}
	sizes := []Size{
		{Width: 60, Height: 10},
		{Width: 30, Height: 6},
		{Width: 10, Height: 3}, // boundary of Size.Valid
		{Width: 0, Height: 0},  // invalid; Render returns ""
	}

	for _, ct := range chartTypes {
		ct := ct
		for _, tc := range cases {
			tc := tc
			for _, sz := range sizes {
				sz := sz
				name := ct + "/" + tc.name
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("panic on %s size=%+v: %v", name, sz, r)
						}
					}()
					_ = Render(ct, DefaultPalette(), tc.table, tc.cfg, sz)
				})
			}
		}
	}
}

func TestRender_TrimsWhitespaceAndCase(t *testing.T) {
	t.Parallel()
	tbl := growingTable(3)
	cfg := api.ChartConfig{LabelKey: "name", YKey: "value"}
	out := Render("  BAR  ", DefaultPalette(), tbl, cfg, Size{Width: 50, Height: 8})
	if out == "" {
		t.Fatalf("expected dispatch on padded mixed-case type")
	}
}

func TestCoerceFloat_RejectsInfNaN(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"Inf", "+Inf", "-Inf", "inf", "NaN", "nan"} {
		if _, ok := coerceFloat(in); ok {
			t.Errorf("coerceFloat(%q) should reject", in)
		}
	}
}
