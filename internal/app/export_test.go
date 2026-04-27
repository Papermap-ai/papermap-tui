package app

import (
	tea "charm.land/bubbletea/v2"

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
