package conversations_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat/conversations"
)

func sampleEntries(n int) []api.ChatHistoryEntry {
	out := make([]api.ChatHistoryEntry, n)
	for i := 0; i < n; i++ {
		out[i] = api.ChatHistoryEntry{
			LLMDataChatID: "chat-" + string(rune('a'+i)),
			Name:          "Chat " + string(rune('A'+i)),
			ModifiedAt:    "2026-04-01T00:00:00Z",
		}
	}
	return out
}

func TestConversationsLoadingState(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Conversations") {
		t.Fatalf("expected title, got %q", view)
	}
	if !strings.Contains(view, "Loading conversations") {
		t.Fatalf("expected loading copy, got %q", view)
	}
}

func TestConversationsRendersEntries(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(3), 1)

	view := m.View(theme.Default(), 80)
	for _, want := range []string{"Chat A", "Chat B", "Chat C", "2026-04-01"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q, got %q", want, view)
		}
	}
}

func TestConversationsEmptyState(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, nil, 0)

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "No prior conversations") {
		t.Fatalf("expected empty-state copy, got %q", view)
	}
}

func TestConversationsErrorState(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetError("network down")

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "network down") {
		t.Fatalf("expected error copy, got %q", view)
	}
}

func TestConversationsEnterEmitsOpenChat(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(2), 1)

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	_, cmd := updated.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected open cmd")
	}
	msg, ok := cmd().(conversations.OpenChatMsg)
	if !ok {
		t.Fatalf("expected OpenChatMsg, got %T", cmd())
	}
	if msg.Chat.LLMDataChatID != "chat-b" {
		t.Fatalf("unexpected chat: %+v", msg.Chat)
	}
}

func TestConversationsEscEmitsCancel(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(1), 1)

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected cancel cmd")
	}
	if _, ok := cmd().(conversations.CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestConversationsCursorWrapsWhenNoMorePages(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(3), 1)

	// Move to bottom.
	for i := 0; i < 2; i++ {
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
		m = updated
	}
	if m.Cursor() != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.Cursor())
	}
	// Next j wraps because no more pages.
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	if cmd != nil {
		t.Fatal("expected no LoadMoreMsg when no pages remain")
	}
	if updated.Cursor() != 0 {
		t.Fatalf("expected cursor wrap to 0, got %d", updated.Cursor())
	}
}

func TestConversationsEmitsLoadMoreAtEnd(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(3), 2) // total 2 pages -> more available

	for i := 0; i < 2; i++ {
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
		m = updated
	}
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	if cmd == nil {
		t.Fatal("expected LoadMoreMsg cmd")
	}
	if _, ok := cmd().(conversations.LoadMoreMsg); !ok {
		t.Fatalf("expected LoadMoreMsg, got %T", cmd())
	}
}

func TestConversationsSetPageAppendsAndDedups(t *testing.T) {
	t.Parallel()

	m := conversations.NewModel()
	m.SetPage(1, sampleEntries(2), 2)

	// Page 2 contains a dup of page 1 entry plus a new one.
	page2 := []api.ChatHistoryEntry{
		{LLMDataChatID: "chat-a", Name: "Chat A"},
		{LLMDataChatID: "chat-c", Name: "Chat C"},
	}
	m.SetPage(2, page2, 2)

	if got := len(m.Entries()); got != 3 {
		t.Fatalf("expected 3 unique entries after page 2, got %d", got)
	}
	if m.Page() != 2 {
		t.Fatalf("expected page 2, got %d", m.Page())
	}
}
