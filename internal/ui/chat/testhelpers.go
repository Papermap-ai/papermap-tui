package chat

// This file exposes a small surface used by tests in other packages
// (notably internal/app). Bubble Tea's external test packages cannot
// see _test.go helpers, so the helpers live in a regular file. They
// are intentionally cheap and side-effect free; production code does
// not call them.

// TextareaSetValueForTest sets the textarea contents directly so tests
// can build up an input state without simulating each keystroke.
func (m *Model) TextareaSetValueForTest(s string) {
	m.textarea.SetValue(s)
}

// TranscriptForTest renders the transcript at the requested width and
// returns the raw string so tests can assert on its contents without
// reaching into private rendering helpers.
func (m Model) TranscriptForTest(width int) string {
	return m.transcriptView(m.theme, width)
}
