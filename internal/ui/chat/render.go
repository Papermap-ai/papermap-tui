package chat

import (
	"strings"

	"github.com/papermap/papermap-tui/internal/theme"
)

func renderRichText(th theme.Theme, width int, content string) string {
	return th.Body.Width(max(width-8, 20)).Render(strings.TrimSpace(content))
}

func renderTable(th theme.Theme, width int, table *Table) string {
	if table == nil || len(table.Columns) == 0 || len(table.Rows) == 0 {
		return ""
	}

	columns := normalizeColumns(table.Columns, table.Rows)
	cellWidth := max((width-12)/max(len(columns), 1), 12)

	lines := []string{formatTableRow(columns, cellWidth), formatTableDivider(len(columns), cellWidth)}
	for _, row := range table.Rows {
		lines = append(lines, formatTableRow(row, cellWidth))
	}

	return th.Muted.Render(strings.Join(lines, "\n"))
}

func normalizeColumns(columns []string, rows [][]string) []string {
	maxColumns := len(columns)
	for _, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}

	normalized := make([]string, maxColumns)
	copy(normalized, columns)
	for i := range normalized {
		if strings.TrimSpace(normalized[i]) == "" {
			normalized[i] = "Column"
		}
	}

	return normalized
}

func formatTableRow(row []string, cellWidth int) string {
	formatted := make([]string, 0, len(row))
	for _, cell := range row {
		formatted = append(formatted, padCell(cell, cellWidth))
	}
	return "| " + strings.Join(formatted, " | ") + " |"
}

func formatTableDivider(columns int, cellWidth int) string {
	parts := make([]string, columns)
	for i := range columns {
		parts[i] = strings.Repeat("-", cellWidth)
	}
	return "|-" + strings.Join(parts, "-+-") + "-|"
}

func padCell(value string, width int) string {
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) > width {
		runes := []rune(trimmed)
		if width > 1 {
			trimmed = string(runes[:width-1]) + "…"
		} else {
			trimmed = string(runes[:width])
		}
	}
	padding := width - len([]rune(trimmed))
	if padding < 0 {
		padding = 0
	}
	return trimmed + strings.Repeat(" ", padding)
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
