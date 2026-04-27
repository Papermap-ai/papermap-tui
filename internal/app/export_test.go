package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

// StartupMsgFromInit runs Init() on the provided model and returns the
// startupMsg produced by the loadStartup command. Test-only; lives in
// export_test.go so it never ships in the binary.
func StartupMsgFromInit(m Model) (tea.Msg, bool) {
	cmd := m.Init()
	if cmd == nil {
		return nil, false
	}
	return findStartupMsg(cmd)
}

// Chat exposes the embedded chat model for tests so they can inspect
// transcript and viewport state after routing messages through Update.
func (m Model) Chat() chat.Model {
	return m.chat
}

// SeedChatForTest places the app on the chat screen with the provided
// transcript messages and a sized viewport. Used to exercise message
// routing (e.g. mouse wheel) without going through the full startup flow.
func (m Model) SeedChatForTest(width, height int, messages ...chat.Message) Model {
	m.screen = screenChat
	m.width = width
	m.height = height
	m.chat, _ = m.chat.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.chat.AppendTestMessages(messages...)
	return m
}

// SetAuthenticatedForTest marks the model as authenticated so tests can
// exercise authenticated-only key paths without going through the
// startup credential flow.
func (m Model) SetAuthenticatedForTest() Model {
	m.authenticated = true
	return m
}

// ScreenName returns the current screen as a string for test assertions.
func (m Model) ScreenName() string {
	return string(m.screen)
}

// SetStreamingForTest forces the model into the in-flight insight state
// without driving the full SubmitMsg/HTTP/SSE pipeline. Used to exercise
// the user-initiated cancel path. The chat textarea is reset so the test
// can later assert the restored prompt.
func (m Model) SetStreamingForTest(prompt string, requestID string, client *api.Client) Model {
	m.client = client
	m.pendingRequestID = requestID
	m.chat.AppendTestMessages(
		chat.Message{Role: "you", Content: prompt},
		chat.Message{Role: "alan", Pending: true},
	)
	m.chat.MarkStreamingForTest()
	return m
}

// PendingRequestID exposes the in-flight request id for tests.
func (m Model) PendingRequestID() string {
	return m.pendingRequestID
}

// CancelNotice exposes any transient cancel notice for tests.
func (m Model) CancelNotice() string {
	return m.cancelNotice
}

// ScreenCommandPalette is the screen identifier for the palette overlay.
const ScreenCommandPalette = string(screenCommandPalette)

// ScreenConversations is the screen identifier for the conversations overlay.
const ScreenConversations = string(screenConversations)

// ScreenChat is the screen identifier for the main chat surface.
const ScreenChat = string(screenChat)

func findStartupMsg(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	msg := cmd()
	switch v := msg.(type) {
	case startupMsg:
		return v, true
	case tea.BatchMsg:
		for _, sub := range v {
			if got, ok := findStartupMsg(sub); ok {
				return got, true
			}
		}
	}
	return nil, false
}
