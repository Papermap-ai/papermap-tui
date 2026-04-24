package api_test

import (
	"reflect"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestChartConfigFromMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   map[string]any
		want api.ChartConfig
	}{
		{
			name: "nil map",
			in:   nil,
			want: api.ChartConfig{},
		},
		{
			name: "empty map",
			in:   map[string]any{},
			want: api.ChartConfig{},
		},
		{
			name: "full backend payload",
			in: map[string]any{
				"title":     "Top Departments",
				"subtitle":  "Counts by Department",
				"x_key":     "department",
				"y_key":     "count",
				"label_key": "department",
				"colors":    []any{"#4e79a7", "#f28e2b"},
				"width":     float64(600),
				"height":    float64(400),
			},
			want: api.ChartConfig{
				Title:    "Top Departments",
				Subtitle: "Counts by Department",
				XKey:     "department",
				YKey:     "count",
				LabelKey: "department",
				Colors:   []string{"#4e79a7", "#f28e2b"},
			},
		},
		{
			name: "tile shape (empty x/y keys)",
			in: map[string]any{
				"title":     "Total Employees",
				"x_key":     "",
				"y_key":     "",
				"label_key": "Total Employees",
			},
			want: api.ChartConfig{
				Title:    "Total Employees",
				LabelKey: "Total Employees",
			},
		},
		{
			name: "drops non-string colors",
			in: map[string]any{
				"colors": []any{"#4e79a7", 42, nil, "  ", "#f28e2b"},
			},
			want: api.ChartConfig{
				Colors: []string{"#4e79a7", "#f28e2b"},
			},
		},
		{
			name: "ignores wrong-type values",
			in: map[string]any{
				"title":  123,
				"x_key":  []any{"oops"},
				"colors": "not-a-list",
			},
			want: api.ChartConfig{},
		},
		{
			name: "trims whitespace",
			in: map[string]any{
				"title": "  spaced  ",
				"x_key": "\tdepartment\n",
			},
			want: api.ChartConfig{
				Title: "spaced",
				XKey:  "department",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := api.ChartConfigFromMap(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ChartConfigFromMap()\n  got:  %+v\n  want: %+v", got, tc.want)
			}
		})
	}
}

func TestInsightResponse_ChartConfig(t *testing.T) {
	t.Parallel()

	resp := api.InsightResponse{
		VisualizationConfig: map[string]any{
			"title": "X",
			"x_key": "month",
			"y_key": "revenue",
		},
	}

	got := resp.Chart()
	want := api.ChartConfig{Title: "X", XKey: "month", YKey: "revenue"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Chart()\n  got:  %+v\n  want: %+v", got, want)
	}
}
