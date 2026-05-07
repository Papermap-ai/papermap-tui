// Conversations and command-palette wiring for the chat screen. The
// palette is the single surface that exposes screen-level commands; one
// of those commands (Conversations) opens a paginated history overlay
// that lets the user load and continue a prior chat. Both surfaces are
// rendered as centered overlays composited over the chat view, mirroring
// the workspace picker.
package app

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/ui/chat"
	"github.com/papermap/papermap-tui/internal/ui/components/palette"
)

// historyPageSize is the per-page request size for the conversations
// overlay. Matches the visible window so a single fetch fills the panel.
const historyPageSize = 10

// conversationsPageSize is the per-page request size when fetching the
// turns of a single chat. The transcript renders the union of pages, so
// a small page minimizes initial latency while still loading enough
// context to be useful.
const conversationsPageSize = 20

// chatHistoryLoadedMsg carries the result of a /chats-history fetch.
// page is 1-indexed.
type chatHistoryLoadedMsg struct {
	page    int
	entries []api.ChatHistoryEntry
	total   int
	err     string
	// sessionExpired routes auth errors through the standard flow.
	sessionExpired bool
}

// conversationsLoadedMsg carries the result of a /chats/{id}/conversations
// fetch when the user opens a prior chat from the conversations overlay.
type conversationsLoadedMsg struct {
	chatID         string
	chatName       string
	entries        []api.ConversationEntry
	totalPages     int
	err            string
	sessionExpired bool
}

// openCommandPalette primes the palette with the chat command catalog and
// switches to the palette overlay screen.
func (m *Model) openCommandPalette() {
	m.palette.SetCommands(chatPaletteCommands())
	m.screen = screenCommandPalette
}

// openConversations primes the conversations overlay in its loading state
// and dispatches the first-page fetch.
func (m *Model) openConversations() tea.Cmd {
	m.conversations.Reset()
	m.screen = screenConversations
	return m.fetchChatHistory(1)
}

// fetchChatHistory builds a tea.Cmd that loads page from the backend and
// returns chatHistoryLoadedMsg.
func (m Model) fetchChatHistory(page int) tea.Cmd {
	if m.client == nil || strings.TrimSpace(m.defaultDashboard) == "" {
		// Without a dashboard we cannot scope history; surface an empty
		// result so the overlay shows the empty-state copy.
		return func() tea.Msg {
			return chatHistoryLoadedMsg{page: page}
		}
	}
	client := m.client
	dashboardID := m.defaultDashboard
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		result, err := client.ListChatHistory(ctx, dashboardID, page, historyPageSize, true)
		if err != nil {
			if _, ok := sessionExpiredFromError(err); ok {
				return chatHistoryLoadedMsg{page: page, err: err.Error(), sessionExpired: true}
			}
			return chatHistoryLoadedMsg{page: page, err: err.Error()}
		}
		return chatHistoryLoadedMsg{
			page:    page,
			entries: result.Chats,
			total:   result.TotalPages,
		}
	}
}

// fetchConversations loads the first page of a chat's conversations and
// returns conversationsLoadedMsg. Subsequent pages are deferred to a
// future change; v1 loads page 1 only on chat open.
func (m Model) fetchConversations(entry api.ChatHistoryEntry) tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return conversationsLoadedMsg{
				chatID:   entry.LLMDataChatID,
				chatName: entry.Name,
				err:      "API client is not ready yet.",
			}
		}
	}
	client := m.client
	chatID := entry.LLMDataChatID
	chatName := entry.Name
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		page, err := client.ListConversations(ctx, chatID, 1, conversationsPageSize)
		if err != nil {
			if _, ok := sessionExpiredFromError(err); ok {
				return conversationsLoadedMsg{
					chatID:         chatID,
					chatName:       chatName,
					err:            err.Error(),
					sessionExpired: true,
				}
			}
			return conversationsLoadedMsg{
				chatID:   chatID,
				chatName: chatName,
				err:      err.Error(),
			}
		}
		return conversationsLoadedMsg{
			chatID:     chatID,
			chatName:   chatName,
			entries:    page.Conversations,
			totalPages: page.TotalPages,
		}
	}
}

// chartBackfillTarget pairs a saved turn's llm_data_id with the index
// of its assistant message in the loaded transcript so the result of an
// async GetChart call can be merged back into the right slot without a
// post-hoc lookup.
type chartBackfillTarget struct {
	llmDataID    string
	messageIndex int
}

// chartBackfilledMsg carries the result of one GetChart fetch dispatched
// after a conversation loads. messageIndex points into the chat
// transcript at the time of dispatch; the handler validates the index
// against MessageCount before applying so a transcript swap mid-flight
// (e.g. user opens another chat) cannot poison the current view.
type chartBackfilledMsg struct {
	chatID       string
	messageIndex int
	response     *api.InsightResponse
	err          string
}

// handleChatHistoryLoaded folds the fetch result into the conversations
// sub-model. The sub-model handles dedup on append.
func (m Model) handleChatHistoryLoaded(msg chatHistoryLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.sessionExpired {
		return m.Update(sessionExpiredMsg{
			reason: "Your session expired. Please sign in again.",
		})
	}
	if msg.err != "" {
		m.conversations.SetError(msg.err)
		return m, nil
	}
	m.conversations.SetPage(msg.page, msg.entries, msg.total)
	return m, nil
}

// handleConversationsLoaded swaps the chat transcript with the loaded
// conversation. On error the conversations overlay is re-shown with an
// error so the user can retry or pick another chat.
func (m Model) handleConversationsLoaded(msg conversationsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.sessionExpired {
		return m.Update(sessionExpiredMsg{
			reason: "Your session expired. Please sign in again.",
		})
	}
	if msg.err != "" {
		// Re-open the conversations overlay so the user sees the
		// failure in context rather than an empty chat screen.
		m.conversations.SetError(msg.err)
		m.screen = screenConversations
		return m, nil
	}
	messages, targets := chatMessagesFromConversations(msg.entries)
	m.cancelInsight()
	m.resetInsightState()
	m.chat.LoadConversation(msg.chatID, messages)
	if name := strings.TrimSpace(msg.chatName); name != "" {
		// Chat name is not surfaced in the chat header today; the load
		// succeeds silently. Future work can wire this through.
		_ = name
	}
	m.screen = screenChat
	return m, m.fetchChartBackfills(msg.chatID, targets)
}

// fetchChartBackfills returns a tea.Batch of GetChart commands, one per
// turn that has a non-empty llm_data_id. Returns nil when the API
// client is missing or no targets need backfilling so callers can pass
// the result directly to a tea.Cmd slot.
func (m Model) fetchChartBackfills(chatID string, targets []chartBackfillTarget) tea.Cmd {
	if m.client == nil || len(targets) == 0 {
		return nil
	}
	client := m.client
	cmds := make([]tea.Cmd, 0, len(targets))
	for _, target := range targets {
		target := target
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			response, err := client.GetChart(ctx, target.llmDataID)
			if err != nil {
				return chartBackfilledMsg{
					chatID:       chatID,
					messageIndex: target.messageIndex,
					err:          err.Error(),
				}
			}
			return chartBackfilledMsg{
				chatID:       chatID,
				messageIndex: target.messageIndex,
				response:     &response,
			}
		})
	}
	return tea.Batch(cmds...)
}

// handleChartBackfilled merges a fetched chart payload into the loaded
// transcript. The chatID guard prevents a late response from mutating a
// transcript the user has since swapped away from. Errors are dropped
// silently because the saved text response remains visible regardless;
// surfacing per-turn fetch failures would create noise without a clear
// recovery action.
func (m Model) handleChartBackfilled(msg chartBackfilledMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" || msg.response == nil {
		return m, nil
	}
	if strings.TrimSpace(msg.chatID) == "" || msg.chatID != m.chat.ChatID() {
		return m, nil
	}
	if msg.messageIndex < 0 || msg.messageIndex >= m.chat.MessageCount() {
		return m, nil
	}
	m.chat.UpdateMessageVisuals(msg.messageIndex, buildAssistantMessage(msg.response))
	return m, nil
}

// chatMessagesFromConversations converts paginated conversation turns
// into chat.Message entries paired with the indices of the assistant
// messages that need an async chart backfill. Each turn is a (user,
// assistant) pair; the assistant message carries the saved thoughts as
// a single completed trace step so the thinking timeline reads
// consistently with live requests. Charts/tables backfill via GetChart
// after the transcript is in view.
func chatMessagesFromConversations(entries []api.ConversationEntry) ([]chat.Message, []chartBackfillTarget) {
	if len(entries) == 0 {
		return nil, nil
	}
	messages := make([]chat.Message, 0, len(entries)*2)
	targets := make([]chartBackfillTarget, 0, len(entries))
	for _, entry := range entries {
		if q := strings.TrimSpace(entry.UserQuery); q != "" {
			messages = append(messages, chat.Message{Role: "you", Content: q})
		}
		assistant := chat.Message{
			Role:    "alan",
			Content: strings.TrimSpace(entry.TextResponse),
		}
		if thoughts := strings.TrimSpace(entry.Thoughts); thoughts != "" {
			assistant.Trace = []chat.TraceStep{{
				Kind:   chat.TraceThought,
				Title:  "Thinking",
				Body:   thoughts,
				Status: "complete",
			}}
			assistant.TraceComplete = true
		}
		messages = append(messages, assistant)
		if id := strings.TrimSpace(entry.LLMDataID); id != "" {
			targets = append(targets, chartBackfillTarget{
				llmDataID:    id,
				messageIndex: len(messages) - 1,
			})
		}
	}
	return messages, targets
}

// dispatchPaletteCommand maps a palette selection to the matching app
// action. Returns the resulting tea.Cmd so the caller can pass it back
// up the Update chain.
func (m *Model) dispatchPaletteCommand(cmd palette.Command) tea.Cmd {
	switch cmd.ID {
	case commandConversations:
		return m.openConversations()
	case commandSwitchWorkspace:
		m.openWorkspacePicker()
		return nil
	case commandSwitchModel:
		m.openModelPicker()
		return nil
	case commandToggleThinking:
		m.screen = screenChat
		m.chat.ToggleThinking()
		return nil
	case commandShellMode:
		m.screen = screenChat
		// Mirror the "!" intercept guard: only latch shell mode when the
		// textarea is empty and nothing else is in flight. Silently
		// no-op otherwise so the palette never strands the user in a
		// partially-typed prompt or mid-stream.
		if m.chat.TextareaIsEmpty() && !m.chat.IsStreaming() && !m.chat.IsShellRunning() && !m.chat.IsShellMode() {
			m.chat.SetShellMode(true)
		}
		return nil
	case commandClearSession:
		m.screen = screenChat
		m.cancelInsight()
		m.resetInsightState()
		m.chat.Clear()
		return nil
	case commandQuit:
		m.screen = screenChat
		return m.openQuitDialog()
	}
	m.screen = screenChat
	return nil
}

// overlayCommandPalette composites the palette modal centered on the
// chat view.
func (m Model) overlayCommandPalette(base string) string {
	return m.centerOverlay(base, m.palette.View(m.theme, m.width))
}

// overlayConversations composites the conversations modal centered on
// the chat view.
func (m Model) overlayConversations(base string) string {
	return m.centerOverlay(base, m.conversations.View(m.theme, m.width))
}
