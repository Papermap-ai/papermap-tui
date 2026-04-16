package chat

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
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
	width          int
	height         int
	textarea       textarea.Model
	messages       []Message
	streaming      bool
	err            string
	chatID         string
	requestID      string
	activeResponse int
	theme          theme.Theme
}

func NewModel(th theme.Theme) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.SetVirtualCursor(true)
	ta.Focus()

	ta.Prompt = ""
	ta.SetWidth(30)
	ta.SetHeight(1)
	ta.MaxHeight = 3
	ta.MinHeight = 1
	ta.DynamicHeight = true

	// Remove cursor line styling like in the example.
	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Base = lipgloss.NewStyle().
		Background(lipgloss.Color("#11111B")).
		Foreground(lipgloss.Color("#F2F5F4"))
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ED8A3"))
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#97A6A8"))

	s.Blurred = s.Focused
	ta.SetStyles(s)

	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return Model{
		textarea:       ta,
		activeResponse: -1,
		theme:          th,
	}
}

// Init returns the initial command for the textarea (cursor blink).
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+l" {
			m.Clear()
			return m, nil
		}

		if m.streaming {
			return m, nil
		}

		if msg.String() == "enter" {
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt == "" {
				m.err = "Enter a question to continue."
				return m, nil
			}
			m.beginRequest(prompt)
			return m, func() tea.Msg { return SubmitMsg{Prompt: prompt} }
		}
	}

	// Delegate all other messages to the textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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
	}
}

func (m *Model) SetStreamTable(table *Table) {
	if table == nil {
		return
	}
	m.ensureAssistantSlot()
	m.messages[m.activeResponse].Table = table
	m.messages[m.activeResponse].Pending = false
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
}

func (m *Model) Clear() {
	m.textarea.Reset()
	m.messages = nil
	m.streaming = false
	m.err = ""
	m.chatID = ""
	m.requestID = ""
	m.activeResponse = -1
}

func (m Model) ChatID() string {
	return m.chatID
}

func (m Model) View(th theme.Theme, workspace string, width int) string {
	if workspace == "" {
		workspace = "Unified Workspace"
	}

	panelWidth := clampWidth(width, 88)
	innerWidth := panelWidth - 6

	// Workspace label.
	workspaceLabel := th.Title.Render("Workspace: " + workspace)

	// Render textarea (prompt is built-in, no external accent bar).
	if innerWidth > 0 {
		m.textarea.SetWidth(innerWidth)
	}
	inputView := m.textarea.View()

	// Key hints.
	hints := th.KeyHint.Render(
		"enter: submit  ·  ctrl+l: clear chat  ·  ctrl+w: switch workspace  ·  ctrl+c: quit",
	)

	if len(m.messages) == 0 {
		// Empty state: logo above, then workspace label, textarea, hints — with spacing.
		logo := components.Logo(th, panelWidth)

		content := lipgloss.JoinVertical(lipgloss.Left,
			workspaceLabel,
			"",
			inputView,
		)

		// Error line if present.
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

		// Join logo and content with more spacing (four blank lines).
		block := lipgloss.JoinVertical(lipgloss.Left, logo, "", "", "", "", content)
		return lipgloss.Place(width, m.height, lipgloss.Center, lipgloss.Center, block)
	}

	// Active chat: transcript + composer at bottom.
	status := "ready"
	if m.streaming {
		status = "streaming"
	}
	if m.err != "" {
		status = "error"
	}

	transcript := m.transcriptView(th, panelWidth)

	composer := lipgloss.JoinVertical(lipgloss.Left,
		workspaceLabel,
		"",
		inputView,
	)

	if m.err != "" {
		composer = lipgloss.JoinVertical(lipgloss.Left,
			composer,
			"",
			th.Error.Render(m.err),
		)
	}

	composer = lipgloss.JoinVertical(lipgloss.Left,
		composer,
		"",
		hints,
	)

	body := th.Panel.Width(panelWidth).Render(strings.Join([]string{
		components.StatusBar(th, "workspace: "+workspace, "stream: "+status),
		"",
		transcript,
		"",
		composer,
	}, "\n"))

	// Return body without logo.
	return body
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
	if message.Role == "you" {
		roleStyle = th.Title
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

	return strings.Join(parts, "\n")
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
