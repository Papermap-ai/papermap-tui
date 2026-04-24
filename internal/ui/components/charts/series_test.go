package charts

import (
	"errors"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestCoerceFloat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in     string
		want   float64
		wantOk bool
	}{
		{"", 0, false},
		{"  ", 0, false},
		{"null", 0, false},
		{"NaN", 0, false},
		{"none", 0, false},
		{"<nil>", 0, false},
		{"42", 42, true},
		{"-3.5", -3.5, true},
		{"1e3", 1000, true},
		{"42%", 42, true},
		{"1,234.5", 1234.5, true},
		{"1,234,567", 1234567, true},
		{"abc", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, ok := coerceFloat(tc.in)
			if ok != tc.wantOk || (ok && got != tc.want) {
				t.Fatalf("coerceFloat(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

func TestExtractSeries_BackendBarShape(t *testing.T) {
	t.Parallel()

	// Mirrors analyzer_prompts.py:469-479 example payload.
	table := &api.InsightTable{
		Columns: []string{"department", "count"},
		Rows: [][]string{
			{"Sales", "40"},
			{"Marketing", "35"},
			{"Engineering", "55"},
		},
	}
	cfg := api.ChartConfig{XKey: "department", YKey: "count", LabelKey: "department"}

	got, err := ExtractSeries(table, cfg)
	if err != nil {
		t.Fatalf("ExtractSeries: %v", err)
	}
	if len(got.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(got.Points))
	}
	if got.Skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", got.Skipped)
	}

	want := []Point{
		{Label: "Sales", X: 0, Y: 40},
		{Label: "Marketing", X: 1, Y: 35},
		{Label: "Engineering", X: 2, Y: 55},
	}
	for i, p := range got.Points {
		if p != want[i] {
			t.Errorf("point[%d] = %+v, want %+v", i, p, want[i])
		}
	}
}

func TestExtractSeries_NumericXAxis(t *testing.T) {
	t.Parallel()

	table := &api.InsightTable{
		Columns: []string{"month_num", "revenue"},
		Rows: [][]string{
			{"1", "1000"},
			{"2", "1500"},
			{"3", "2000"},
		},
	}
	cfg := api.ChartConfig{XKey: "month_num", YKey: "revenue"}

	got, err := ExtractSeries(table, cfg)
	if err != nil {
		t.Fatalf("ExtractSeries: %v", err)
	}
	for i, p := range got.Points {
		if p.X != float64(i+1) {
			t.Errorf("point[%d].X = %v, want %v", i, p.X, float64(i+1))
		}
	}
}

func TestExtractSeries_SkipsNonNumericRows(t *testing.T) {
	t.Parallel()

	table := &api.InsightTable{
		Columns: []string{"label", "value"},
		Rows: [][]string{
			{"a", "10"},
			{"b", ""},
			{"c", "n/a"},
			{"d", "20"},
		},
	}
	cfg := api.ChartConfig{LabelKey: "label", YKey: "value"}

	got, err := ExtractSeries(table, cfg)
	if err != nil {
		t.Fatalf("ExtractSeries: %v", err)
	}
	if len(got.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(got.Points))
	}
	if got.Skipped != 2 {
		t.Fatalf("expected 2 skipped, got %d", got.Skipped)
	}
}

func TestExtractSeries_EmptyOrMissingColumns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		table *api.InsightTable
	}{
		{"nil table", nil},
		{"no columns", &api.InsightTable{}},
		{"no rows", &api.InsightTable{Columns: []string{"a"}, Rows: nil}},
		{
			"all non-numeric",
			&api.InsightTable{
				Columns: []string{"a", "b"},
				Rows:    [][]string{{"x", "y"}, {"p", "q"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ExtractSeries(tc.table, api.ChartConfig{})
			if !errors.Is(err, ErrNoNumericData) {
				t.Fatalf("err = %v, want ErrNoNumericData", err)
			}
		})
	}
}

func TestExtractSeries_FallbackToFirstNumericColumn(t *testing.T) {
	t.Parallel()

	table := &api.InsightTable{
		Columns: []string{"name", "score"},
		Rows: [][]string{
			{"Alice", "92"},
			{"Bob", "85"},
		},
	}
	// No YKey: extractor must pick "score".
	got, err := ExtractSeries(table, api.ChartConfig{})
	if err != nil {
		t.Fatalf("ExtractSeries: %v", err)
	}
	if len(got.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(got.Points))
	}
	if got.Points[0].Y != 92 || got.Points[0].Label != "Alice" {
		t.Errorf("unexpected first point: %+v", got.Points[0])
	}
}

func TestExtractSeries_LabelFallsBackToRowIndex(t *testing.T) {
	t.Parallel()

	table := &api.InsightTable{
		Columns: []string{"value"},
		Rows: [][]string{
			{"10"},
			{"20"},
		},
	}
	got, err := ExtractSeries(table, api.ChartConfig{YKey: "value"})
	if err != nil {
		t.Fatalf("ExtractSeries: %v", err)
	}
	if got.Points[0].Label != "0" || got.Points[1].Label != "1" {
		t.Errorf("expected row-index labels, got %+v", got.Points)
	}
}
