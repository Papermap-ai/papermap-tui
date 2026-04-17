package chat

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

type Message struct {
	Role    string
	Content string
	Table   *Table
	Pending bool
}

type Model struct {
	width            int
	height           int
	textarea         textarea.Model
	viewport         viewport.Model
	messages         []Message
	streaming        bool
	err              string
	chatID           string
	requestID        string
	activeResponse   int
	theme            theme.Theme
	contentNeedsSync bool
}

func NewModel(th theme.Theme) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.SetVirtualCursor(true)
	ta.Focus()

	ta.Prompt = ""
	ta.SetWidth(30)
	ta.SetHeight(2)
	ta.MaxHeight = 5
	ta.MinHeight = 2
	ta.DynamicHeight = true

	s := ta.Styles()
	inputBg := lipgloss.Color("#11111B")
	s.Focused.CursorLine = lipgloss.NewStyle().Background(inputBg)
	s.Focused.Base = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(lipgloss.Color("#F2F5F4"))
	s.Focused.Text = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(lipgloss.Color("#F2F5F4"))
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Background(inputBg)
	s.Focused.Prompt = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(lipgloss.Color("#2ED8A3"))
	s.Focused.Placeholder = lipgloss.NewStyle().
		Background(inputBg).
		Foreground(lipgloss.Color("#97A6A8"))

	s.Blurred = s.Focused
	ta.SetStyles(s)

	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return Model{
		textarea:       ta,
		viewport:       vp,
		activeResponse: -1,
		theme:          th,
	}
}

// Init returns the initial command for the textarea (cursor blink) and viewport.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.viewport.Init())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.contentNeedsSync = true
		m.updateViewportDimensions()
		return m, nil

	case tea.MouseWheelMsg:
		// Route mouse wheel directly to viewport when we have messages.
		// Skip content sync check for smooth scrolling.
		if len(m.messages) > 0 {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}
		return m, nil

	case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		// Route all mouse events to viewport when we have messages for smooth interaction.
		if len(m.messages) > 0 {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			if vpCmd != nil {
				return m, vpCmd
			}
		}
		// Fall through to delegate to textarea for input area clicks.

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+l" {
			m.Clear()
			m.contentNeedsSync = true
			return m, nil
		}

		if m.streaming {
			return m, nil
		}

		// Handle scroll keys - route to viewport when not editing multi-line input.
		switch msg.String() {
		case "pgup":
			m.viewport.PageUp()
			return m, nil
		case "pgdown":
			m.viewport.PageDown()
			return m, nil
		}

		if msg.String() == "enter" {
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt == "" {
				m.err = "Enter a question to continue."
				m.updateViewportDimensions()
				return m, nil
			}
			m.beginRequest(prompt)
			m.contentNeedsSync = true
			m.syncViewportContent()
			return m, func() tea.Msg { return SubmitMsg{Prompt: prompt} }
		}
	}

	// Update viewport content if needed.
	if m.contentNeedsSync {
		m.syncViewportContent()
	}

	// Delegate to textarea for typing.
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	if taCmd != nil {
		cmds = append(cmds, taCmd)
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

func (m *Model) AppendStreamText(text string) {
	if strings.TrimSpace(text) == "" && text == "" {
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
	m.contentNeedsSync = true
	m.scrollToBottom()
}

func (m *Model) SetStreamTable(table *Table) {
	if table == nil {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Table = table
	m.messages[m.activeResponse].Pending = false
	m.contentNeedsSync = true
	m.scrollToBottom()
}

func (m *Model) CompleteStream() {
	m.streaming = false
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) &&
		strings.TrimSpace(m.messages[m.activeResponse].Content) == "" &&
		m.messages[m.activeResponse].Table == nil {
		m.messages[m.activeResponse].Content = "No content returned."
	}
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		m.messages[m.activeResponse].Pending = false
	}
	m.activeResponse = -1
}

func (m *Model) FailStream(err string) {
	m.streaming = false
	m.err = strings.TrimSpace(err)
	if m.activeResponse >= 0 && m.activeResponse < len(m.messages) {
		if strings.TrimSpace(m.messages[m.activeResponse].Content) == "" {
			m.messages[m.activeResponse].Content = "Request failed."
		}
		m.messages[m.activeResponse].Pending = false
	}
	m.activeResponse = -1
}

func (m *Model) ReplaceHistory(messages []Message) {
	m.messages = append([]Message(nil), messages...)
	m.streaming = false
	m.err = ""
	m.activeResponse = -1
	for i := range m.messages {
		m.messages[i].Pending = false
	}
	m.contentNeedsSync = true
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

	replaced := append([]Message(nil), m.messages[:start]...)
	for _, message := range messages {
		message.Pending = false
		replaced = append(replaced, message)
	}

	m.messages = replaced
	m.streaming = false
	m.err = ""
	m.activeResponse = -1
	m.contentNeedsSync = true
}

func (m *Model) Clear() {
	m.textarea.Reset()
	m.messages = nil
	m.streaming = false
	m.err = ""
	m.chatID = ""
	m.requestID = ""
	m.activeResponse = -1
	m.contentNeedsSync = true
}

func (m Model) ChatID() string {
	return m.chatID
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
		m.textarea.SetWidth(max(innerWidth-2, 10))
	}
	bgStyle := lipgloss.NewStyle().Background(th.InputBg)
	taView := padLinesToWidth(bgStyle, m.textarea.Width(), m.textarea.View())
	inputView := addLeftBar(th.Accent, taView)

	hints := th.KeyHint.Render(
		"enter: submit  ·  ctrl+l: clear chat  ·  ctrl+w: switch workspace  ·  ctrl+c: quit",
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
	hints := th.KeyHint.Render(
		"enter: submit  ·  ctrl+l: clear  ·  ctrl+w: switch  ·  ctrl+c: quit",
	)

	// Input area.
	bgStyle := lipgloss.NewStyle().Background(th.InputBg)
	taView := padLinesToWidth(bgStyle, m.textarea.Width(), m.textarea.View())
	inputView := addLeftBar(th.Accent, taView)

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
	m.streaming = true
	m.messages = append(m.messages,
		Message{Role: "you", Content: prompt},
		Message{Role: "alan", Pending: true},
	)
	m.activeResponse = len(m.messages) - 1
	m.contentNeedsSync = true
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

	blocks := make([]string, 0, len(m.messages))
	for _, message := range m.messages {
		blocks = append(blocks, renderMessage(th, width, message))
	}

	return strings.Join(blocks, "\n\n")
}

func renderMessage(th theme.Theme, width int, message Message) string {
	roleStyle := th.Accent
	barColor := th.Accent
	if message.Role == "you" {
		roleStyle = th.Title
		barColor = th.Title
	}

	body := strings.TrimSpace(message.Content)
	if body == "" && message.Pending {
		body = th.Muted.Render("Streaming response...")
	} else if body == "" {
		body = th.Muted.Render("No content.")
	} else {
		body = renderRichText(th, width, body)
	}

	parts := []string{roleStyle.Render(strings.ToUpper(message.Role)), body}
	if message.Table != nil {
		parts = append(parts, renderTable(th, width, message.Table))
	}

	content := strings.Join(parts, "\n")

	// Add left accent bar.
	return addLeftBar(barColor, content)
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
	m.viewport.GotoBottom()
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
	hints := m.theme.KeyHint.Render(
		"enter: submit  ·  ctrl+l: clear  ·  ctrl+w: switch  ·  ctrl+c: quit",
	)
	hintsHeight := lipgloss.Height(hints)

	// Calculate input height.
	m.textarea.SetWidth(max(width-4-2, 10))
	bgStyle := lipgloss.NewStyle().Background(m.theme.InputBg)
	taView := padLinesToWidth(bgStyle, m.textarea.Width(), m.textarea.View())
	inputView := addLeftBar(m.theme.Accent, taView)
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
	m.contentNeedsSync = false
}
