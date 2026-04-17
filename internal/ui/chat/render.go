package chat

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// trailingGapPattern matches a run of trailing whitespace that may be wrapped
// in (or followed by) ANSI reset sequences. It captures the trimmed trailing
// segment so we can re-paint it with the desired background.
var trailingGapPattern = regexp.MustCompile(`(?:\x1b\[[0-9;]*m)*[ \t]+(?:\x1b\[[0-9;]*m)*$`)

func addLeftBar(barStyle lipgloss.Style, content string) string {
	bar := barStyle.Render("▎")
	lines := strings.Split(content, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = bar + " " + line
	}
	return strings.Join(result, "\n")
}

// padLinesToWidth ensures every line in content is a solid rectangle of width
// columns by re-painting the trailing unstyled gap with bgStyle. The upstream
// textarea emits end-of-buffer lines where the trailing whitespace lies
// outside any style block, so the terminal renders it with the default
// background. We strip that bare gap and re-render it with bgStyle so the
// entire input block shares one background color. A leading reset ensures
// that any unterminated attributes (e.g. the reverse-video virtual cursor)
// do not bleed into the re-painted gap.
func padLinesToWidth(bgStyle lipgloss.Style, width int, content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		stripped := trailingGapPattern.ReplaceAllString(line, "")
		visible := lipgloss.Width(stripped)
		gap := width - visible
		if gap < 0 {
			gap = 0
		}
		lines[i] = stripped + "\x1b[m" + bgStyle.Render(strings.Repeat(" ", gap))
	}
	return strings.Join(lines, "\n")
}

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
