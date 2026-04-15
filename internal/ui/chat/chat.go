package chat

import (
	"strings"
	"unicode/utf8"

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
	prompt         string
	messages       []Message
	streaming      bool
	err            string
	chatID         string
	requestID      string
	activeResponse int
}

func NewModel() Model {
	return Model{activeResponse: -1}
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

		switch msg.String() {
		case "backspace":
			m.prompt = trimLastRune(m.prompt)
			return m, nil
		case "enter":
			prompt := strings.TrimSpace(m.prompt)
			if prompt == "" {
				m.err = "Enter a question to continue."
				return m, nil
			}

			m.beginRequest(prompt)
			return m, func() tea.Msg { return SubmitMsg{Prompt: prompt} }
		}

		if msg.Key().Text != "" {
			m.err = ""
			m.prompt += msg.Key().Text
		}
	}

	return m, nil
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
	m.prompt = ""
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
	status := "ready"
	if m.streaming {
		status = "streaming"
	}
	if m.err != "" {
		status = "error"
	}

	transcript := m.transcriptView(th, panelWidth)
	composer := m.composerView(th, panelWidth)

	body := th.Panel.Width(panelWidth).Render(strings.Join([]string{
		components.StatusBar(th, "workspace: "+workspace, "stream: "+status),
		"",
		transcript,
		"",
		composer,
	}, "\n"))

	return strings.Join([]string{components.Logo(th), "", body}, "\n")
}

func (m *Model) beginRequest(prompt string) {
	m.err = ""
	m.prompt = ""
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

func (m Model) composerView(th theme.Theme, width int) string {
	prompt := m.prompt
	if prompt == "" {
		prompt = th.Muted.Render("Ask Papermap...")
	}

	lines := []string{
		th.Title.Render("Prompt"),
		"",
		th.Body.Render("> " + prompt),
	}

	if m.err != "" {
		lines = append(lines, "", th.Error.Render(m.err))
	}

	if m.streaming {
		lines = append(
			lines,
			"",
			th.Muted.Render("Waiting for current stream to finish before sending another prompt."),
		)
	}

	lines = append(
		lines,
		"",
		th.KeyHint.Render(
			"Enter submit  •  Ctrl+L clear chat  •  Ctrl+W switch workspace  •  Ctrl+C quit",
		),
	)

	return lipgloss.NewStyle().MaxWidth(width - 8).Render(strings.Join(lines, "\n"))
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

func trimLastRune(value string) string {
	if value == "" {
		return value
	}

	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 {
		return ""
	}

	return value[:len(value)-size]
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
