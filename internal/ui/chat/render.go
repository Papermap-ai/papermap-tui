package chat

import (
	"regexp"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/papermap/papermap-tui/internal/theme"
)

// glamourCache memoizes TermRenderer instances per word-wrap width so we
// avoid rebuilding the markdown pipeline on every redraw. The renderers
// hold mutable internal state (e.g. glamour's BlockStack) and are not safe
// for concurrent use, so all access goes through glamourMu.
var (
	glamourMu    sync.Mutex
	glamourCache = map[int]*glamour.TermRenderer{}
)

func glamourRenderer(width int) *glamour.TermRenderer {
	if r, ok := glamourCache[width]; ok {
		return r
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	glamourCache[width] = r
	return r
}

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
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	wrap := max(width-8, 20)
	// Lookup and Render are both protected because the cached renderer
	// holds mutable state and is not safe for concurrent use.
	glamourMu.Lock()
	defer glamourMu.Unlock()
	if r := glamourRenderer(wrap); r != nil {
		if out, err := r.Render(trimmed); err == nil {
			// Glamour adds leading/trailing blank lines for breathing room
			// inside a full document; inside a chat bubble that wastes
			// vertical space, so trim them.
			return strings.Trim(out, "\n")
		}
	}
	return th.Body.Width(wrap).Render(trimmed)
}

// maxCellContentWidth caps the visible width of any single table cell.
// Cells are truncated with an ellipsis past this width, even when the
// available column width could fit more characters. The cap keeps wide
// strings (URLs, long descriptions) from forcing the entire table to
// overflow the viewport.
const maxCellContentWidth = 20

func renderTable(th theme.Theme, t *Table) string {
	if t == nil || len(t.Columns) == 0 || len(t.Rows) == 0 {
		return ""
	}

	columns := normalizeColumns(t.Columns, t.Rows)

	// Truncate every cell upfront so column widths stay bounded; lipgloss
	// table handles padding, alignment, and borders for us.
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = truncateCell(col, maxCellContentWidth)
	}

	rows := make([][]string, len(t.Rows))
	for i, row := range t.Rows {
		cells := make([]string, len(columns))
		for j := range columns {
			value := ""
			if j < len(row) {
				value = row[j]
			}
			cells[j] = truncateCell(value, maxCellContentWidth)
		}
		rows[i] = cells
	}

	headerStyle := th.Title.Padding(0, 1)
	cellStyle := th.Body.Padding(0, 1)
	borderStyle := th.Muted

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		BorderRow(false).
		BorderColumn(true).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	return tbl.Render()
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

// truncateCell trims whitespace and shortens value to at most width runes,
// appending an ellipsis when content was cut.
func truncateCell(value string, width int) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= width {
		return trimmed
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
