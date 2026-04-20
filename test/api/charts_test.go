package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// chartFixture returns an HTTP body matching the backend envelope around
// a chart_type+data payload, so tests exercise the same decoding path as
// the real /charts/stream endpoint.
func chartFixture(chartType string, dataJSON string, text string) string {
	return `{
		"message": "ok",
		"success": true,
		"status_code": 200,
		"data": {
			"llm_data_id": "llm-1",
			"response_type": "chart",
			"text_response": ` + jsonString(text) + `,
			"chart_type": ` + jsonString(chartType) + `,
			"data": ` + dataJSON + `
		}
	}`
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestBuildDataRowsTablePreservesKeyOrder(t *testing.T) {
	t.Parallel()

	// Note the keys are intentionally id, name, count - alphabetical
	// sort would yield count, id, name. We assert insertion order wins.
	body := chartFixture("table",
		`[{"id":1,"name":"Alpha","count":10},{"id":2,"name":"Beta","count":20}]`,
		"Here are the rows.",
	)

	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	table := api.BuildDataRowsTable(resp.RawDataJSON)
	if table == nil {
		t.Fatal("expected table from data array, got nil")
	}

	want := []string{"id", "name", "count"}
	if !equalStrings(table.Columns, want) {
		t.Fatalf("columns out of order: got %v, want %v", table.Columns, want)
	}

	if len(table.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(table.Rows))
	}
	if table.Rows[0][0] != "1" || table.Rows[0][1] != "Alpha" || table.Rows[0][2] != "10" {
		t.Fatalf("row 0 mismatch: %v", table.Rows[0])
	}
	if table.Rows[1][0] != "2" || table.Rows[1][1] != "Beta" || table.Rows[1][2] != "20" {
		t.Fatalf("row 1 mismatch: %v", table.Rows[1])
	}
}

func TestBuildDataRowsTableSparseRows(t *testing.T) {
	t.Parallel()

	body := chartFixture("table",
		`[{"id":1,"name":"Alpha"},{"id":2,"extra":"X"}]`,
		"",
	)

	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	table := api.BuildDataRowsTable(resp.RawDataJSON)
	if table == nil {
		t.Fatal("expected table, got nil")
	}

	want := []string{"id", "name", "extra"}
	if !equalStrings(table.Columns, want) {
		t.Fatalf("columns: got %v, want %v", table.Columns, want)
	}
	if table.Rows[0][2] != "" {
		t.Fatalf("expected empty cell for missing extra, got %q", table.Rows[0][2])
	}
	if table.Rows[1][1] != "" {
		t.Fatalf("expected empty cell for missing name, got %q", table.Rows[1][1])
	}
}

func TestBuildDataRowsTableEmptyArray(t *testing.T) {
	t.Parallel()

	body := chartFixture("table", `[]`, "")
	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := api.BuildDataRowsTable(resp.RawDataJSON); got != nil {
		t.Fatalf("expected nil table for empty array, got %+v", got)
	}
}

func TestBuildTileFromFirstRow(t *testing.T) {
	t.Parallel()

	body := chartFixture("tile", `[{"employee_total":56}]`, "Total headcount.")
	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tile := api.BuildTile(resp.RawDataJSON)
	if tile == nil {
		t.Fatal("expected tile, got nil")
	}
	if tile.Label != "employee_total" {
		t.Fatalf("label: got %q want %q", tile.Label, "employee_total")
	}
	if tile.Value != "56" {
		t.Fatalf("value: got %q want %q", tile.Value, "56")
	}
}

func TestBuildTileEmptyData(t *testing.T) {
	t.Parallel()

	body := chartFixture("tile", `[]`, "")
	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := api.BuildTile(resp.RawDataJSON); got != nil {
		t.Fatalf("expected nil tile for empty data, got %+v", got)
	}
}

func TestInsightResponseCapturesRawDataJSON(t *testing.T) {
	t.Parallel()

	body := chartFixture("table", `[{"a":1}]`, "")
	var resp api.InsightResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.RawDataJSON) == 0 {
		t.Fatal("expected RawDataJSON to be populated")
	}
	trimmed := strings.TrimSpace(string(resp.RawDataJSON))
	if !strings.HasPrefix(trimmed, "[") {
		t.Fatalf("expected raw data array, got %q", trimmed)
	}
}

func TestGetChart(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/analytics/charts/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(chartFixture("tile", `[{"revenue":12345.67}]`, "Q1 revenue.")))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := client.GetChart(context.Background(), "llm-99")
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	if resp.ChartType != "tile" {
		t.Fatalf("chart_type: got %q", resp.ChartType)
	}
	tile := api.BuildTile(resp.RawDataJSON)
	if tile == nil || tile.Label != "revenue" {
		t.Fatalf("tile not parsed: %+v", tile)
	}
}

func equalStrings(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
