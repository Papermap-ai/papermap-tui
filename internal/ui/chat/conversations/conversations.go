// Package conversations renders the chat history overlay for the chat
// screen. It is a focused sub-model emitting messages the parent app
// translates into API calls (load more pages) and chat transitions
// (open a selected chat).
package conversations

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/theme"
)

// LoadMoreMsg is emitted when the cursor reaches the end of the loaded
// list and there are still pages remaining. The parent should fetch the
// next page and feed the result back via SetPage.
type LoadMoreMsg struct{}

// OpenChatMsg is emitted when the user confirms a selection. The parent
// app fetches the chat's conversations and swaps the chat transcript.
type OpenChatMsg struct {
	Chat api.ChatHistoryEntry
}

// CancelMsg is emitted when the user dismisses the overlay.
type CancelMsg struct{}

const (
	pageSize  = 10
	maxPanelW = 84
	minPanelW = 56
)

// panelWidthFor mirrors the workspace picker so overlays feel consistent
// across the app.
func panelWidthFor(screenWidth int) int {
	if screenWidth <= 0 {
		return minPanelW
	}
	width := (screenWidth - 6) * 4 / 5
	if width > maxPanelW {
		width = maxPanelW
	}
	if width < minPanelW {
		width = screenWidth - 6
		if width < 32 {
			width = 32
		}
	}
	return width
}

// Model is the conversations history overlay state.
type Model struct {
	entries     []api.ChatHistoryEntry
	cursor      int
	page        int
	totalPages  int
	loading     bool
	loadMessage string
	err         string
	// loadingMore latches while a page-N fetch is in flight to suppress
	// duplicate LoadMoreMsg emissions.
	loadingMore bool
}

// NewModel returns an empty conversations model in its initial loading
// state. The parent should call SetPage with page=1 once the first
// fetch completes.
func NewModel() Model {
	return Model{
		loading:     true,
		loadMessage: "Loading conversations...",
		page:        0,
	}
}

// Reset returns the model to its initial loading state. Useful when the
// dashboard or workspace context changes.
func (m *Model) Reset() {
	m.entries = nil
	m.cursor = 0
	m.page = 0
	m.totalPages = 0
	m.loading = true
	m.loadMessage = "Loading conversations..."
	m.err = ""
	m.loadingMore = false
}

// SetPage applies a page of results. page is 1-indexed; pass page=1 for
// the initial load and incremented values for subsequent appends. The
// model deduplicates entries by chat id so retries are safe.
func (m *Model) SetPage(page int, entries []api.ChatHistoryEntry, totalPages int) {
	m.loading = false
	m.loadingMore = false
	m.err = ""
	m.totalPages = totalPages
	if page <= 1 {
		m.entries = append([]api.ChatHistoryEntry(nil), entries...)
		m.page = 1
		m.cursor = 0
		return
	}

	seen := make(map[string]struct{}, len(m.entries))
	for _, entry := range m.entries {
		seen[entry.LLMDataChatID] = struct{}{}
	}
	for _, entry := range entries {
		if _, ok := seen[entry.LLMDataChatID]; ok {
			continue
		}
		m.entries = append(m.entries, entry)
		seen[entry.LLMDataChatID] = struct{}{}
	}
	m.page = page
}

// SetError marks the model as failed to load. message is short and shown
// in place of the entries.
func (m *Model) SetError(message string) {
	m.loading = false
	m.loadingMore = false
	m.err = strings.TrimSpace(message)
}

// Entries returns the loaded chat entries. Useful for tests.
func (m Model) Entries() []api.ChatHistoryEntry {
	return m.entries
}

// Cursor returns the current cursor index.
func (m Model) Cursor() int {
	return m.cursor
}

// Page returns the current loaded page (1-indexed). Zero before the
// first load completes.
func (m Model) Page() int {
	return m.page
}

// hasMore reports whether more pages can still be fetched.
func (m Model) hasMore() bool {
	if m.totalPages <= 0 {
		return false
	}
	return m.page < m.totalPages
}

// Update routes key input. Emits OpenChatMsg, LoadMoreMsg, or CancelMsg
// via the returned cmd.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		return m, func() tea.Msg { return CancelMsg{} }

	case "enter":
		if m.loading || len(m.entries) == 0 {
			return m, nil
		}
		if m.cursor < 0 || m.cursor >= len(m.entries) {
			return m, nil
		}
		selected := m.entries[m.cursor]
		return m, func() tea.Msg { return OpenChatMsg{Chat: selected} }

	case "down", "j", "ctrl+n":
		if len(m.entries) == 0 {
			return m, nil
		}
		// At the bottom: trigger load-more if available, otherwise wrap.
		if m.cursor == len(m.entries)-1 {
			if m.hasMore() && !m.loadingMore {
				m.loadingMore = true
				return m, func() tea.Msg { return LoadMoreMsg{} }
			}
			m.cursor = 0
			return m, nil
		}
		m.cursor++
		return m, nil

	case "up", "k":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = (m.cursor - 1 + len(m.entries)) % len(m.entries)
		return m, nil

	case "home", "g":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = 0
		return m, nil

	case "end", "G":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.cursor = len(m.entries) - 1
		return m, nil
	}

	return m, nil
}

// View renders the overlay panel.
func (m Model) View(th theme.Theme, screenWidth int) string {
	width := panelWidthFor(screenWidth)
	header := th.Title.Render("Conversations")

	var body string
	switch {
	case m.loading:
		body = th.Muted.Render(m.loadMessage)
	case m.err != "":
		body = th.Error.Render(m.err)
	case len(m.entries) == 0:
		body = th.Muted.Render("No prior conversations for this dashboard.")
	default:
		body = m.renderEntries(th, width-8)
	}

	footer := m.renderFooter(th)

	parts := []string{header, "", body}
	if footer != "" {
		parts = append(parts, "", footer)
	}
	content := strings.Join(parts, "\n")

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.LogoColorA).
		Padding(1, 3).
		Width(width)

	return panel.Render(content)
}

func (m Model) renderEntries(th theme.Theme, innerWidth int) string {
	if innerWidth < 24 {
		innerWidth = 24
	}

	cursorStyle := lipgloss.NewStyle().Foreground(th.LogoColorA).Bold(true)

	const visibleRows = pageSize
	start := 0
	end := len(m.entries)
	if end > visibleRows {
		// Slide the window so the cursor stays in view.
		start = m.cursor - visibleRows/2
		if start < 0 {
			start = 0
		}
		if start+visibleRows > len(m.entries) {
			start = len(m.entries) - visibleRows
		}
		end = start + visibleRows
	}

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := m.entries[i]
		isCursor := i == m.cursor

		var prefix string
		if isCursor {
			prefix = cursorStyle.Render("›")
		} else {
			prefix = " "
		}

		title := strings.TrimSpace(entry.Name)
		if title == "" {
			title = entry.LLMDataChatID
		}

		titleStyled := th.Body.Render(truncate(title, innerWidth-12))
		if isCursor {
			titleStyled = th.Accent.Render(truncate(title, innerWidth-12))
		}

		meta := strings.TrimSpace(entry.ModifiedAt)
		if meta == "" {
			meta = strings.TrimSpace(entry.CreatedAt)
		}
		// Backend timestamps are RFC3339; just keep the date portion for
		// the row to avoid noise. Fall back to the raw string if the
		// format is unexpected.
		if len(meta) >= 10 && meta[4] == '-' && meta[7] == '-' {
			meta = meta[:10]
		}
		metaStyled := ""
		if meta != "" {
			metaStyled = th.Muted.Render(meta)
		}

		left := fmt.Sprintf("%s %s", prefix, titleStyled)
		lines = append(lines, alignTwoColumns(left, metaStyled, innerWidth))
	}

	if m.loadingMore {
		lines = append(lines, th.Muted.Render("  Loading more..."))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFooter(th theme.Theme) string {
	if m.loading {
		return th.KeyHint.Render("Esc cancel")
	}
	if m.err != "" {
		return th.KeyHint.Render("Esc close")
	}
	if len(m.entries) == 0 {
		return th.KeyHint.Render("Esc close")
	}
	hints := "↑↓ navigate  •  Enter open  •  Esc cancel"
	if m.totalPages > 1 {
		hints = fmt.Sprintf(
			"↑↓ navigate  •  Enter open  •  Esc cancel  •  %d loaded · page %d/%d",
			len(m.entries), m.page, m.totalPages,
		)
	}
	return th.KeyHint.Render(hints)
}

func alignTwoColumns(left, right string, width int) string {
	if right == "" {
		return left
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncate(s string, max int) string {
	if max <= 0 {
		return s
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	// Trim by rune count as a cheap approximation; sufficient for ASCII
	// chat names which are the common case.
	runes := []rune(s)
	if len(runes) <= max-1 {
		return s
	}
	return string(runes[:max-1]) + "…"
}
