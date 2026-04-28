package chat

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

type SubmitMsg struct {
	Prompt string
}

type Table struct {
	Columns []string
	Rows    [][]string
}

// Tile holds a single-metric value derived from a chart_type=tile response.
// FormatConfig is the raw visualization_config from the backend so the
// renderer can apply currency/percent/integer formatting consistently.
type Tile struct {
	Label        string
	Value        string
	FormatConfig map[string]any
}

// Chart carries the source table and parsed config needed to render a
// bar or pie chart at view time. Stored on the message rather than
// pre-rendered so the renderer can react to viewport width changes
// without re-fetching the response.
type Chart struct {
	Type   string
	Table  *api.InsightTable
	Config api.ChartConfig
}

type Message struct {
	Role      string
	Content   string
	Table     *Table
	Tile      *Tile
	Chart     *Chart
	ChartType string
	// EmptyData is set when the response had a chart but the data array
	// was empty. The renderer surfaces a "(no rows)" placeholder in that
	// case so users see the response landed but had nothing to display.
	EmptyData bool
	Pending   bool
	// Error, when non-empty, marks the message as a failed or cancelled
	// assistant turn. The renderer replaces the body with a pink ERROR
	// badge followed by this text and suppresses the role label, trace,
	// and any chart/table/tile content. Used for both user-initiated
	// cancels and stream failures so both render the same way inline.
	Error string
	// Trace holds Alan's reasoning timeline (thoughts and tool calls)
	// streamed alongside the final answer. Rendered above the body so
	// the trace reads as supporting context.
	Trace []TraceStep
	// TraceComplete latches once the request finishes so the renderer
	// can switch from the live preview to the full trace.
	TraceComplete bool
}

type Model struct {
	width          int
	height         int
	textarea       textarea.Model
	viewport       viewport.Model
	spinner        spinner.Model
	messages       []Message
	streaming      bool
	streamStatus   string
	err            string
	chatID         string
	requestID      string
	activeResponse int
	theme          theme.Theme
	lastTAHeight   int
	// showThinking is the sticky preference for the reasoning trace
	// display, toggled with ctrl+t. When true the renderer shows the
	// full trace (live ticker while streaming, full timeline after
	// completion). When false it shows a muted single-line preview
	// while streaming and hides the trace entirely after completion.
	showThinking bool
	// userScrolled latches when the user moves the viewport away from
	// the bottom (mouse wheel, page-up, etc). Streaming updates respect
	// this flag and stop forcing the viewport to the bottom so trace
	// inspection isn't yanked away as new chunks arrive.
	userScrolled bool
	// selectedModel is the display label for the active LLM model,
	// shown as a badge pinned bottom-left inside the input box. Empty
	// hides the badge row entirely.
	selectedModel string
	// pastes tracks every collapsed paste in the current draft. Reset
	// whenever the textarea is cleared (submit, ctrl+l, conversation
	// load) so chip ids never bleed across prompts.
	pastes pasteRegistry
}

func NewModel(th theme.Theme) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.SetVirtualCursor(true)
	ta.Focus()

	ta.Prompt = ""
	ta.SetWidth(25)
	ta.SetHeight(2)
	ta.MaxHeight = 5
	ta.MinHeight = 3
	ta.DynamicHeight = true

	s := ta.Styles()
	inputBg := th.InputBg
	s.Focused.CursorLine = lipgloss.NewStyle().Background(inputBg)
	s.Focused.Base = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(th.TextColor)
	s.Focused.Text = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(th.TextColor)
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Background(inputBg)
	s.Focused.Prompt = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(th.LogoColorA)
	s.Focused.Placeholder = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(th.MutedColor)

	s.Blurred = s.Focused
	ta.SetStyles(s)

	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(th.LogoColorA)

	return Model{
		textarea:       ta,
		viewport:       vp,
		spinner:        sp,
		activeResponse: -1,
		theme:          th,
		showThinking:   true,
		pastes:         newPasteRegistry(),
	}
}

// Init returns the initial command for the textarea (cursor blink) and viewport.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.viewport.Init(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.streaming {
			m.syncViewportContent()
		}
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportDimensions()
		m.syncViewportContent()
		m.scrollToBottom()
		return m, nil

	case tea.MouseWheelMsg:
		// Route mouse wheel directly to viewport when we have messages.
		if len(m.messages) > 0 {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			m.noteUserScroll()
			return m, vpCmd
		}
		return m, nil

	case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		if len(m.messages) > 0 {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			m.noteUserScroll()
			if vpCmd != nil {
				return m, vpCmd
			}
		}

	case tea.PasteMsg:
		// Large pastes collapse into a "[Pasted ~N lines]" chip so the
		// prompt stays readable. The token stored in the textarea is
		// expanded back to the original text on submit.
		text := msg.Content
		if shouldCollapsePaste(text) {
			token, _ := m.pastes.add(text)
			m.textarea.InsertString(token)
			if h := m.textarea.Height(); h != m.lastTAHeight {
				m.lastTAHeight = h
				m.updateViewportDimensions()
			}
			m.syncViewportContent()
			return m, nil
		}
		// Small pastes flow through to the textarea unchanged.

	case tea.KeyPressMsg:
		key := msg.String()

		// Scroll keys should always work, even while streaming. Arrow-based
		// bindings use Shift to avoid conflicting with textarea cursor nav.
		switch key {
		case "shift+up":
			m.viewport.ScrollUp(1)
			m.noteUserScroll()
			return m, nil
		case "shift+down":
			m.viewport.ScrollDown(1)
			m.noteUserScroll()
			return m, nil
		case "pgup", "shift+pgup":
			m.viewport.PageUp()
			m.noteUserScroll()
			return m, nil
		case "pgdown", "shift+pgdown":
			m.viewport.PageDown()
			m.noteUserScroll()
			return m, nil
		case "home", "shift+home":
			m.viewport.GotoTop()
			m.noteUserScroll()
			return m, nil
		case "end", "shift+end":
			m.viewport.GotoBottom()
			m.userScrolled = false
			return m, nil
		case "ctrl+d":
			m.viewport.HalfPageDown()
			m.noteUserScroll()
			return m, nil
		}

		if key == "ctrl+t" {
			m.showThinking = !m.showThinking
			m.syncViewportContent()
			return m, nil
		}

		// Ctrl+L wipes the entire prompt textarea so the user can
		// quickly reset their input. This is the canonical "clear" key
		// in terminal UIs and is documented in internal/ui/AGENTS.md
		// as the chat clear binding. It works through every common
		// terminal + tmux config (no kitty/CSI-u protocol required),
		// unlike Cmd/Super+Backspace which is silently dropped by
		// older terminals and tmux without extended-keys.
		if key == "ctrl+l" {
			m.textarea.Reset()
			m.pastes.reset()
			m.err = ""
			m.updateViewportDimensions()
			m.syncViewportContent()
			return m, nil
		}

		// Only block submit while streaming; other input still flows.
		if key == "enter" {
			if m.streaming {
				return m, nil
			}
			raw := m.textarea.Value()
			expanded := strings.TrimSpace(m.pastes.expand(raw))
			if expanded == "" {
				m.err = "Enter a question to continue."
				m.updateViewportDimensions()
				m.syncViewportContent()
				return m, nil
			}
			m.beginRequest(expanded)
			return m, tea.Batch(
				func() tea.Msg { return SubmitMsg{Prompt: expanded} },
				m.spinner.Tick,
			)
		}

		// Backspace immediately after a chip token deletes the whole
		// chip in one keystroke instead of nibbling the synthetic
		// "[#p:N]" sequence character by character.
		if key == "backspace" {
			value := m.textarea.Value()
			byteOffset := cursorByteOffset(value, m.textarea.Line(), m.textarea.Column())
			if chip, start, end, ok := m.pastes.chipBeforeCursor(value, byteOffset); ok {
				// Rebuild the buffer in two steps so the cursor lands
				// at the chip's old start position. SetValue resets
				// the cursor to end-of-content, so we set the prefix
				// first (cursor at end of prefix == old start) and
				// then InsertString the suffix without moving the
				// cursor past it would land us at end again — instead
				// we splice via a single SetValue and then walk the
				// cursor back to the prefix end.
				prefix := value[:start]
				suffix := value[end:]
				m.textarea.SetValue(prefix + suffix)
				m.pastes.remove(chip.id)
				placeCursorAtRuneOffset(&m.textarea, prefix+suffix, utf8Len(prefix))
				if h := m.textarea.Height(); h != m.lastTAHeight {
					m.lastTAHeight = h
					m.updateViewportDimensions()
				}
				m.syncViewportContent()
				return m, nil
			}
		}
	}

	// Delegate to textarea for typing.
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	if taCmd != nil {
		cmds = append(cmds, taCmd)
	}

	// If the textarea height changed, recompute viewport dimensions so it
	// doesn't overlap the input area.
	if h := m.textarea.Height(); h != m.lastTAHeight {
		m.lastTAHeight = h
		m.updateViewportDimensions()
		m.syncViewportContent()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) SetStreamingIDs(chatID string, requestID string) {
	if strings.TrimSpace(chatID) != "" {
		m.chatID = strings.TrimSpace(chatID)
	}
	if strings.TrimSpace(requestID) != "" {
		m.requestID = strings.TrimSpace(requestID)
	}
}

// SetModel sets the display label for the active LLM model badge shown
// inside the input box. Pass an empty string to hide the badge row.
func (m *Model) SetModel(name string) {
	m.selectedModel = strings.TrimSpace(name)
}

// SetStreamStatus sets the ephemeral status line shown alongside the spinner
// while an insight request is in flight. Pass an empty string to clear.
func (m *Model) SetStreamStatus(status string) {
	next := strings.TrimSpace(status)
	if next == m.streamStatus {
		return
	}
	m.streamStatus = next
	m.syncViewportContent()
}

func (m *Model) AppendStreamText(text string) {
	if text == "" {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Content += text
	if strings.TrimSpace(m.messages[m.activeResponse].Content) != "" {
		m.messages[m.activeResponse].Pending = false
	}
	if m.err != "" {
		m.err = ""
		m.updateViewportDimensions()
	}
	m.syncViewportContent()
	m.scrollToBottom()
}

func (m *Model) SetStreamTable(table *Table) {
	if table == nil {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Table = table
	m.messages[m.activeResponse].Pending = false
	m.syncViewportContent()
	m.scrollToBottom()
}

// MergeStreamThoughtDelta appends a streamed reasoning chunk to the active
// assistant slot's trace. complete signals the final delta of the thought
// block so the next chunk in the same iteration starts a fresh entry.
func (m *Model) MergeStreamThoughtDelta(iteration int, delta string, complete bool) {
	if delta == "" && !complete {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Trace = MergeThoughtDelta(
		m.messages[m.activeResponse].Trace,
		iteration,
		delta,
		complete,
	)
	m.syncViewportContent()
	m.scrollToBottom()
}

// AppendStreamTrace appends a single trace step to the active assistant
// slot. Used for tool_call_announced and tool_call_args_complete events.
func (m *Model) AppendStreamTrace(step TraceStep) {
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Trace = append(m.messages[m.activeResponse].Trace, step)
	m.syncViewportContent()
	m.scrollToBottom()
}

// PairToolOutput attaches output / status / duration to the matching tool
// call step (by tool_call_id) in the active assistant slot.
func (m *Model) PairToolOutput(toolCallID string, output string, status string, durationMS float64) {
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Trace = AttachToolOutput(
		m.messages[m.activeResponse].Trace,
		toolCallID,
		output,
		status,
		durationMS,
	)
	m.syncViewportContent()
	m.scrollToBottom()
}

// AppendStreamToolOutputContent appends streamed tool_output content to
// the matching tool call step (by tool_call_id).
func (m *Model) AppendStreamToolOutputContent(toolCallID string, content string) {
	if content == "" {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Trace = AppendToolOutputContent(
		m.messages[m.activeResponse].Trace,
		toolCallID,
		content,
	)
	m.syncViewportContent()
	m.scrollToBottom()
}

// ToggleThinking flips the sticky reasoning-trace visibility preference.
// Exposed so callers (tests, future menu actions) can drive the toggle
// without going through a key event.
func (m *Model) ToggleThinking() {
	m.showThinking = !m.showThinking
	m.syncViewportContent()
}

// ShowThinking reports the current sticky reasoning-trace visibility.
func (m Model) ShowThinking() bool {
	return m.showThinking
}

func (m *Model) CompleteStream() {
	m.streaming = false
	m.streamStatus = ""
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		msg := m.messages[m.activeResponse]
		hasContent := strings.TrimSpace(msg.Content) != "" ||
			msg.Table != nil || msg.Tile != nil ||
			msg.EmptyData || msg.ChartType != ""
		if !hasContent {
			m.messages[m.activeResponse].Content = "No content returned."
		}
		m.messages[m.activeResponse].Pending = false
		if len(m.messages[m.activeResponse].Trace) > 0 {
			m.messages[m.activeResponse].TraceComplete = true
		}
	}
	m.activeResponse = -1
	m.syncViewportContent()
	m.scrollToBottom()
}

func (m *Model) FailStream(err string) {
	m.streaming = false
	m.streamStatus = ""
	trimmed := strings.TrimSpace(err)
	if trimmed == "" {
		trimmed = "Request failed."
	}
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		m.messages[m.activeResponse] = Message{
			Role:  "alan",
			Error: trimmed,
		}
	}
	m.activeResponse = -1
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

func (m *Model) ReplaceLastAssistant(messages []Message) {
	if len(messages) == 0 {
		return
	}

	start := len(m.messages)
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		start = m.activeResponse
	}
	if start > len(m.messages) {
		start = len(m.messages)
	}

	// Preserve the active assistant slot's accumulated trace so the
	// thinking timeline survives the swap to the final answer body.
	// The first replacement message inherits the prior trace and is
	// marked complete so the visibility toggle treats it as eligible.
	var carriedTrace []TraceStep
	carryTrace := false
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		prior := m.messages[m.activeResponse]
		if len(prior.Trace) > 0 {
			carriedTrace = prior.Trace
			carryTrace = true
		}
	}

	replaced := append([]Message(nil), m.messages[:start]...)
	for i, message := range messages {
		message.Pending = false
		if i == 0 && carryTrace && len(message.Trace) == 0 {
			message.Trace = carriedTrace
			message.TraceComplete = true
		}
		replaced = append(replaced, message)
	}

	m.messages = replaced
	m.streaming = false
	m.err = ""
	m.activeResponse = -1
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

func (m *Model) Clear() {
	m.textarea.Reset()
	m.pastes.reset()
	m.messages = nil
	m.streaming = false
	m.streamStatus = ""
	m.err = ""
	m.chatID = ""
	m.requestID = ""
	m.activeResponse = -1
	m.userScrolled = false
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

// CancelStream tears down the active streaming slot in response to a
// user-initiated cancel. The originating user prompt stays in the
// transcript; the pending assistant placeholder is converted into an
// inline error message ("request cancelled") so the user sees the turn
// ended deliberately. Streaming flags are cleared but the textarea is
// left untouched so the user keeps whatever they were typing next.
func (m *Model) CancelStream() {
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		m.messages[m.activeResponse] = Message{
			Role:  "alan",
			Error: "request cancelled",
		}
	}
	m.streaming = false
	m.streamStatus = ""
	m.activeResponse = -1
	m.requestID = ""
	m.textarea.Focus()
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

// LastUserPrompt returns the content of the most recent user message
// in the transcript, or empty string if none exists. Retained as a
// small introspection helper used by tests.
func (m Model) LastUserPrompt() string {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "you" {
			return m.messages[i].Content
		}
	}
	return ""
}

func (m Model) ChatID() string {
	return m.chatID
}

// TextareaIsEmpty reports whether the prompt textarea has no user input.
// The parent app uses this to decide whether keys like "/" should open
// the command palette or be passed through to the textarea as a literal
// character.
func (m Model) TextareaIsEmpty() bool {
	return strings.TrimSpace(m.textarea.Value()) == ""
}

// TextareaValue returns the current contents of the prompt textarea.
// Used by tests that need to assert on restored prompts after cancel.
func (m Model) TextareaValue() string {
	return m.textarea.Value()
}

// IsStreaming reports whether an insight request is currently in flight.
// The parent uses this to suppress overlay openers while streaming so
// the user does not lose mid-stream context.
func (m Model) IsStreaming() bool {
	return m.streaming
}

// LoadConversation replaces the transcript with messages from a previously
// saved chat and binds chatID so the next prompt threads onto the same
// backend chat. Resets streaming state and the textarea so the user lands
// in a clean prompt-ready view. Pass an empty messages slice to swap to
// an existing chat with no prior turns.
func (m *Model) LoadConversation(chatID string, messages []Message) {
	m.textarea.Reset()
	m.pastes.reset()
	m.streaming = false
	m.streamStatus = ""
	m.err = ""
	m.activeResponse = -1
	m.userScrolled = false
	m.chatID = strings.TrimSpace(chatID)
	m.requestID = ""
	if len(messages) == 0 {
		m.messages = nil
	} else {
		m.messages = append([]Message(nil), messages...)
	}
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

// UpdateMessageVisuals merges chart/table/tile payload fields from src
// into the message at idx. Used by the parent app to backfill saved
// chart data after a conversation loads with text-only stubs. The
// existing message's Role, Pending flag, and reasoning Trace are
// preserved; Content is overwritten only when src.Content is non-empty
// so the saved text_response wins over an empty backfill.
func (m *Model) UpdateMessageVisuals(idx int, src Message) {
	if idx < 0 || idx >= len(m.messages) {
		return
	}
	target := &m.messages[idx]
	if strings.TrimSpace(src.Content) != "" {
		target.Content = src.Content
	}
	if src.Table != nil {
		target.Table = src.Table
	}
	if src.Tile != nil {
		target.Tile = src.Tile
	}
	if src.Chart != nil {
		target.Chart = src.Chart
	}
	if strings.TrimSpace(src.ChartType) != "" {
		target.ChartType = src.ChartType
	}
	if src.EmptyData {
		target.EmptyData = true
	}
	m.syncViewportContent()
}

// MessageCount reports the number of transcript messages. Used by the
// parent app to validate indices before calling UpdateMessageVisuals.
func (m Model) MessageCount() int {
	return len(m.messages)
}

// ViewportYOffset returns the transcript viewport's current vertical scroll
// position. Useful for tests and callers that need to inspect scroll state.
func (m Model) ViewportYOffset() int {
	return m.viewport.YOffset()
}

// ViewportTotalLines returns the total number of rendered transcript lines.
func (m Model) ViewportTotalLines() int {
	return m.viewport.TotalLineCount()
}

// AppendTestMessages appends fully-formed transcript messages and refreshes
// the viewport. Intended for tests that need a populated, scrollable
// transcript without driving the streaming pipeline.
func (m *Model) AppendTestMessages(messages ...Message) {
	if len(messages) == 0 {
		return
	}
	m.messages = append(m.messages, messages...)
	m.updateViewportDimensions()
	m.syncViewportContent()
}

// MarkStreamingForTest flips the streaming flag and points
// activeResponse at the last message in the transcript so CancelStream
// can find the pending assistant slot. Test-only seam used by the
// cancel-path tests to simulate an in-flight request.
func (m *Model) MarkStreamingForTest() {
	m.streaming = true
	if len(m.messages) > 0 {
		m.activeResponse = len(m.messages) - 1
	}
}

func (m Model) View(th theme.Theme, workspace string, width int) string {
	if workspace == "" {
		workspace = "Unified Workspace"
	}

	// Empty state: centered logo + input.
	if len(m.messages) == 0 {
		return m.emptyView(th, workspace, width)
	}

	// Active chat: header + scrollable viewport + pinned input.
	return m.activeView(th, workspace, width)
}

func (m Model) emptyView(th theme.Theme, workspace string, width int) string {
	panelWidth := clampWidth(width, 88)
	innerWidth := panelWidth - 6

	logo := components.Logo(th, panelWidth)

	workspaceLabel := th.Title.Render("Workspace: " + workspace)

	if innerWidth > 0 {
		m.textarea.SetWidth(max(innerWidth-3, 10))
	}
	inputView := m.renderInput(th)

	hints := lipgloss.PlaceHorizontal(
		panelWidth,
		lipgloss.Center,
		th.KeyHint.Render(
			"/ : commands  ·  tab: cycle model  ·  ctrl+w: switch workspace  ·  ctrl+c: quit",
		),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		workspaceLabel,
		"",
		inputView,
	)

	if m.err != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			th.Error.Render(m.err),
		)
	}

	if m.streaming {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			th.Muted.Render("Waiting for stream to finish..."),
		)
	}

	content = lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		hints,
	)

	block := lipgloss.JoinVertical(lipgloss.Left, logo, "", "", "", "", content)
	return lipgloss.Place(width, m.height, lipgloss.Center, lipgloss.Center, block)
}

func (m Model) activeView(th theme.Theme, workspace string, width int) string {
	// Header: logo + workspace label (2 lines).
	logoLine := components.SmallRender(th, width-4)
	workspaceLine := th.Title.Render("Workspace: " + workspace)
	header := lipgloss.JoinVertical(lipgloss.Right, logoLine, workspaceLine)

	// Key hints.
	hints := th.KeyHint.Render(thinkingHint(m.showThinking, m.streaming))

	// Input area.
	inputView := m.renderInput(th)

	// Assemble: header, viewport, input, error (if any), hints.
	sections := []string{
		header,
		"",
		m.viewport.View(),
		"",
		inputView,
	}

	if m.err != "" {
		sections = append(sections, "", th.Error.Render(m.err))
	}

	sections = append(sections, "", hints)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) beginRequest(prompt string) {
	m.err = ""
	m.textarea.Reset()
	m.pastes.reset()
	m.streaming = true
	m.userScrolled = false
	m.messages = append(m.messages,
		Message{Role: "you", Content: prompt},
		Message{Role: "alan", Pending: true},
	)
	m.activeResponse = len(m.messages) - 1
	m.updateViewportDimensions()
	m.syncViewportContent()
	m.scrollToBottom()
}

func (m *Model) ensureAssistantSlot() {
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		return
	}
	m.messages = append(m.messages, Message{Role: "alan", Pending: true})
	m.activeResponse = len(m.messages) - 1
}

func (m Model) transcriptView(th theme.Theme, width int) string {
	if len(m.messages) == 0 {
		return strings.Join([]string{
			th.Title.Render("Ask Papermap"),
			"",
			th.Body.Render("Type a question below to start an insight request."),
			th.Muted.Render("Live output streams into this transcript."),
		}, "\n")
	}

	spinnerFrame := m.spinner.View()
	status := m.streamStatus
	blocks := make([]string, 0, len(m.messages))
	for i, message := range m.messages {
		// Only show the live status line on the active pending assistant
		// slot. All other messages render normally.
		msgStatus := ""
		if i == m.activeResponse && message.Pending {
			msgStatus = status
		}
		blocks = append(blocks, renderMessage(th, width, message, spinnerFrame, msgStatus, m.showThinking))
	}

	return strings.Join(blocks, "\n\n")
}

func renderMessage(th theme.Theme, width int, message Message, spinnerFrame string, status string, showThinking bool) string {
	// Failed/cancelled turns render as a compact ERROR badge + message,
	// no role label, no trace, no chart/table/tile, with a red bar so
	// the failure stands out at a glance.
	if strings.TrimSpace(message.Error) != "" {
		badge := th.ErrorBadge.Render("ERROR")
		body := th.Body.Render(strings.TrimSpace(message.Error))
		line := lipgloss.JoinHorizontal(lipgloss.Top, badge, "  ", body)
		return addLeftBar(th.Error, line)
	}

	roleStyle := th.Accent
	barColor := th.Accent
	if message.Role == "you" {
		roleStyle = th.Title
		barColor = th.Title
	}

	body := strings.TrimSpace(message.Content)
	if body == "" && message.Pending {
		label := "Thinking..."
		if strings.TrimSpace(status) != "" {
			label = strings.TrimSpace(status)
		}
		body = th.Muted.Render(spinnerFrame + " " + label)
	} else if body == "" && message.Tile == nil && message.Table == nil && message.Chart == nil && !message.EmptyData {
		body = th.Muted.Render("No content.")
	} else if body != "" {
		body = renderRichText(th, width, body)
	}

	parts := []string{roleStyle.Render(strings.ToUpper(message.Role))}

	// Trace renders right after the role label so the reasoning timeline
	// reads as supporting context above the actual answer body. A blank
	// line follows so the answer visually detaches from the trace.
	if trace := renderTrace(th, width, message, showThinking); trace != "" {
		parts = append(parts, trace, "")
	}

	// Tile renders next so the headline metric is the first thing the
	// user sees in the answer body; the narrative reads as supporting
	// context.
	if message.Tile != nil {
		format := components.TileFormatFromConfig(message.Tile.FormatConfig)
		tile := components.RenderTile(th, max(width-8, 20), message.Tile.Label, message.Tile.Value, format)
		if tile != "" {
			parts = append(parts, tile)
		}
	}

	if body != "" {
		parts = append(parts, body)
	}

	// A blank line between the narrative body and any data visualization
	// (chart, table, badge) so dense visuals do not crowd the prose above.
	hasVisual := message.EmptyData || message.Table != nil ||
		message.Chart != nil || (message.ChartType != "" && message.Tile == nil)
	if body != "" && hasVisual {
		parts = append(parts, "")
	}

	switch {
	case message.EmptyData:
		parts = append(parts, th.Muted.Render("(no rows)"))
	case message.Table != nil:
		parts = append(parts, renderTable(th, message.Table))
	case message.Chart != nil:
		if rendered := renderChart(th, width, message.Chart); rendered != "" {
			parts = append(parts, rendered)
		}
	case message.ChartType != "" && message.Tile == nil:
		if badge := components.ChartBadge(th, message.ChartType); badge != "" {
			parts = append(parts, badge)
		}
	}

	content := strings.Join(parts, "\n")

	// Add left accent bar.
	return addLeftBar(barColor, content)
}

// thinkingHint returns the active-view bottom hint string with the
// current thinking-trace visibility state appended so the keybinding's
// effect is obvious without an experiment. While streaming, the hint
// surfaces the cancel binding instead of the static command list.
func thinkingHint(showThinking bool, streaming bool) string {
	if streaming {
		return "esc: cancel  ·  ctrl+t: thinking [" + onOff(showThinking) + "]  ·  ctrl+c: quit"
	}
	return "/ : commands  ·  tab: cycle model  ·  ctrl+t: thinking [" + onOff(showThinking) + "]  ·  ctrl+w: switch  ·  ctrl+c: quit"
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func clampWidth(width int, fallback int) int {
	if width <= 0 {
		return fallback
	}
	if width < 40 {
		return width
	}
	if width < fallback {
		return width - 4
	}
	return fallback
}

func (m *Model) scrollToBottom() {
	if m.userScrolled {
		return
	}
	m.viewport.GotoBottom()
}

// noteUserScroll updates the sticky-scroll flag based on the viewport's
// current position. Called after any input that moves the viewport so
// streaming updates know whether to keep auto-scrolling.
func (m *Model) noteUserScroll() {
	m.userScrolled = !m.viewport.AtBottom()
}

func (m *Model) updateViewportDimensions() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	const topPadding = 1
	const bottomPadding = 1

	// Calculate header height.
	width := m.width
	logoLine := components.SmallRender(m.theme, width-4)
	workspaceLine := m.theme.Title.Render("Workspace: Unified Workspace")
	header := lipgloss.JoinVertical(lipgloss.Right, logoLine, workspaceLine)
	headerHeight := lipgloss.Height(header)

	// Calculate hints height.
	hints := m.theme.KeyHint.Render(thinkingHint(m.showThinking, m.streaming))
	hintsHeight := lipgloss.Height(hints)

	// Calculate input height.
	m.textarea.SetWidth(max(width-4-2-1, 10))
	inputView := m.renderInput(m.theme)
	inputHeight := lipgloss.Height(inputView)

	// Calculate error height if present.
	errorHeight := 0
	if m.err != "" {
		errorView := m.theme.Error.Render(m.err)
		errorHeight = lipgloss.Height(errorView) + 1 // +1 for blank line separator
	}

	// Calculate viewport height = total - all other components.
	// Components: header, blank, viewport, blank, input, [blank+error], blank, hints
	usedHeight := headerHeight + 1 + inputHeight + errorHeight + 1 + hintsHeight + topPadding + bottomPadding + 1
	viewportHeight := m.height - usedHeight
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	contentWidth := width - 4

	if m.viewport.Width() != contentWidth {
		m.viewport.SetWidth(contentWidth)
	}
	if m.viewport.Height() != viewportHeight {
		m.viewport.SetHeight(viewportHeight)
	}
}

func (m *Model) syncViewportContent() {
	if m.width <= 0 {
		return
	}
	contentWidth := m.width - 4
	transcript := m.transcriptView(m.theme, contentWidth)
	m.viewport.SetContent(transcript)
}

// renderInput composes the prompt input box: the textarea padded to its
// configured width, optionally followed by a model badge row, prefixed
// with the accent left bar plus an extra inner gutter so text does not
// hug the bar. Centralized so the three render sites cannot drift; the
// badge row attaches here so it inherits the InputBg and the continuous
// left bar.
func (m Model) renderInput(th theme.Theme) string {
	bgStyle := lipgloss.NewStyle().Background(th.InputBg)
	width := m.textarea.Width()
	body := m.pastes.decorate(m.textarea.View(), th)
	if badge := m.renderModelBadge(th, width); badge != "" {
		body = body + "\n" + badge
	}
	// One extra left column inside the InputBg rectangle creates
	// breathing room between the accent bar and the text content.
	gutter := bgStyle.Render(" ")
	body = prefixLines(body, gutter)
	taView := padLinesToWidth(bgStyle, width+1, body)
	return addLeftBar(th.InputAccent, taView)
}

// renderModelBadge returns the single-line "Model · <name>" badge styled
// to sit bottom-left inside the input box. "Model" and the dot separator
// stay muted; the model name takes the soft brand accent so it reads as
// the live status value (mirrors patterns like "Build · Claude Opus 4.7").
// Returns "" when no model is selected or when the available width is
// too small to render anything meaningful.
func (m Model) renderModelBadge(th theme.Theme, width int) string {
	name := strings.TrimSpace(m.selectedModel)
	if name == "" || width < 10 {
		return ""
	}
	bg := th.InputBg
	label := truncateBadge("Model", name, width)
	muted := th.KeyHint.Background(bg)
	accent := lipgloss.NewStyle().Foreground(th.LogoColorB).Background(bg).Bold(true)

	prefix, slug, hasSlug := strings.Cut(label, "\x00")
	if !hasSlug {
		return muted.Render(prefix)
	}
	return muted.Render(prefix) + accent.Render(slug)
}

// truncateBadge composes "<prefix> · <slug>" clamped to width display
// columns and returns it with a NUL byte separating the muted prefix
// (label + dot + spaces) from the accent slug so the caller can paint
// each half in its own color. When the slug must be cut, the truncated
// slug stays in the second segment so it keeps the accent color. When
// even the prefix alone overflows, returns just the truncated prefix
// with no NUL separator.
func truncateBadge(label, slug string, width int) string {
	if width <= 0 {
		return ""
	}
	prefix := label + " · "
	full := prefix + slug
	if lipgloss.Width(full) <= width {
		return prefix + "\x00" + slug
	}
	// Slug too long: keep prefix intact, ellipsize the slug.
	if lipgloss.Width(prefix)+1 <= width {
		runes := []rune(slug)
		for i := len(runes); i > 0; i-- {
			candidate := prefix + string(runes[:i-1]) + "…"
			if lipgloss.Width(candidate) <= width {
				return prefix + "\x00" + string(runes[:i-1]) + "…"
			}
		}
	}
	// Even the prefix does not fit; fall back to a clipped label so the
	// badge area is never blank.
	runes := []rune(label)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i-1]) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return "…"
}
