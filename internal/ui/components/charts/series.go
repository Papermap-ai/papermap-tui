package charts

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/papermap/papermap-tui/internal/api"
)

// Point is a single (label, x, y) tuple ready for rendering. Label is the
// human-readable category or series identifier. X and Y are coerced to
// float64; renderers that only need one axis can ignore the other.
type Point struct {
	Label string
	X     float64
	Y     float64
}

// Series is the result of resolving an InsightTable + ChartConfig into
// renderer-ready data. Skipped counts rows that could not be coerced so
// renderers can surface "(N rows skipped)" footers when useful.
type Series struct {
	Points  []Point
	Skipped int
}

// ErrNoNumericData indicates that no row in the table produced a usable
// numeric value. Callers should render an unavailable notice rather than
// an empty chart frame.
var ErrNoNumericData = errors.New("charts: no numeric data in series")

// ExtractSeries resolves an InsightTable into renderer-ready points using
// the keys in cfg. Behavior:
//
//   - Label comes from cfg.LabelKey if set, otherwise cfg.XKey, otherwise
//     the first non-numeric column, otherwise the row index as a string.
//   - X is coerced from cfg.XKey if numeric. When XKey is missing or not
//     numeric, X is set to the zero-based row index so line/scatter can
//     still plot.
//   - Y is coerced from cfg.YKey. When YKey is missing, the first numeric
//     column is used. Rows whose Y cannot be coerced are skipped.
//
// Returns ErrNoNumericData when every row was skipped.
func ExtractSeries(table *api.InsightTable, cfg api.ChartConfig) (Series, error) {
	if table == nil || len(table.Rows) == 0 || len(table.Columns) == 0 {
		return Series{}, ErrNoNumericData
	}

	colIndex := indexColumns(table.Columns)
	yCol, yIdx := resolveYColumn(table, colIndex, cfg.YKey)
	if yIdx < 0 {
		return Series{}, ErrNoNumericData
	}

	labelIdx := resolveLabelColumn(table, colIndex, cfg.LabelKey, cfg.XKey, yCol)
	xIdx := -1
	if cfg.XKey != "" {
		if i, ok := colIndex[cfg.XKey]; ok && i != yIdx {
			xIdx = i
		}
	}

	out := Series{Points: make([]Point, 0, len(table.Rows))}
	for rowIdx, row := range table.Rows {
		y, ok := coerceFloat(cellAt(row, yIdx))
		if !ok {
			out.Skipped++
			continue
		}

		x := float64(rowIdx)
		if xIdx >= 0 {
			if xVal, ok := coerceFloat(cellAt(row, xIdx)); ok {
				x = xVal
			}
		}

		label := ""
		if labelIdx >= 0 {
			label = strings.TrimSpace(cellAt(row, labelIdx))
		}
		if label == "" {
			label = strconv.Itoa(rowIdx)
		}

		out.Points = append(out.Points, Point{Label: label, X: x, Y: y})
	}

	if len(out.Points) == 0 {
		return Series{}, ErrNoNumericData
	}
	return out, nil
}

// coerceFloat converts a raw cell string to float64. Accepts integers,
// floats, and percent-suffixed values like "42%". Empty strings, "null",
// and "nan" are rejected. Negative numbers and scientific notation work
// because strconv.ParseFloat handles them.
func coerceFloat(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	switch strings.ToLower(trimmed) {
	case "null", "nan", "none", "<nil>":
		return 0, false
	}
	// Strip a single trailing % for convenience; the value is returned
	// as a percent (e.g. "42%" -> 42.0), not divided by 100.
	candidate := strings.TrimSuffix(trimmed, "%")
	candidate = strings.ReplaceAll(candidate, ",", "")
	v, err := strconv.ParseFloat(candidate, 64)
	if err != nil {
		return 0, false
	}
	// Reject Inf and NaN explicitly. ParseFloat happily accepts "inf",
	// "-Inf", and similar, which would poison every downstream scaling
	// calculation (xyBounds, projectY, bar widths, pie shares).
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// indexColumns returns a map from column name to position. When the same
// column name appears twice (rare but possible in ad-hoc backend payloads)
// the first occurrence wins.
func indexColumns(cols []string) map[string]int {
	out := make(map[string]int, len(cols))
	for i, c := range cols {
		if _, exists := out[c]; !exists {
			out[c] = i
		}
	}
	return out
}

// resolveYColumn returns (column name, column index) for the y axis,
// preferring cfg.YKey when set and falling back to the first column whose
// values are numeric.
func resolveYColumn(table *api.InsightTable, colIndex map[string]int, yKey string) (string, int) {
	if yKey != "" {
		if i, ok := colIndex[yKey]; ok {
			return yKey, i
		}
	}
	for i, name := range table.Columns {
		if columnIsNumeric(table, i) {
			return name, i
		}
	}
	return "", -1
}

// resolveLabelColumn picks a label column index, preferring labelKey, then
// xKey (when distinct from the y column), then the first non-numeric
// column, then -1 to signal "use row index".
func resolveLabelColumn(table *api.InsightTable, colIndex map[string]int, labelKey, xKey, yCol string) int {
	if labelKey != "" {
		if i, ok := colIndex[labelKey]; ok {
			return i
		}
	}
	if xKey != "" && xKey != yCol {
		if i, ok := colIndex[xKey]; ok {
			return i
		}
	}
	for i, name := range table.Columns {
		if name == yCol {
			continue
		}
		if !columnIsNumeric(table, i) {
			return i
		}
	}
	return -1
}

// columnIsNumeric reports whether a majority of non-empty cells in the
// column at idx coerce to float64. The "majority" rule tolerates the
// occasional missing/null value without rejecting the whole column.
func columnIsNumeric(table *api.InsightTable, idx int) bool {
	if idx < 0 || idx >= len(table.Columns) {
		return false
	}
	numeric := 0
	nonEmpty := 0
	for _, row := range table.Rows {
		cell := cellAt(row, idx)
		if strings.TrimSpace(cell) == "" {
			continue
		}
		nonEmpty++
		if _, ok := coerceFloat(cell); ok {
			numeric++
		}
	}
	if nonEmpty == 0 {
		return false
	}
	return numeric*2 >= nonEmpty
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}
