package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

func TestChatSubmitStartsStreamingTranscript(t *testing.T) {
	t.Parallel()

	model := chat.NewModel()
	for _, key := range []string{"h", "i"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key}))
		model = updated
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	msg := cmd()
	submit, ok := msg.(chat.SubmitMsg)
	if !ok {
		t.Fatalf("expected chat.SubmitMsg, got %T", msg)
	}
	if submit.Prompt != "hi" {
		t.Fatalf("expected prompt hi, got %q", submit.Prompt)
	}

	view := model.View(theme.Default(), "Unified Workspace", 100)
	if !strings.Contains(view, "YOU") || !strings.Contains(view, "Streaming response") {
		t.Fatalf("expected optimistic transcript, got %q", view)
	}
	if !strings.Contains(view, "stream: streaming") {
		t.Fatalf("expected streaming status, got %q", view)
	}

	model.AppendStreamText("hello")
	model.CompleteStream()
	view = model.View(theme.Default(), "Unified Workspace", 100)
	if !strings.Contains(view, "hello") {
		t.Fatalf("expected streamed content in view, got %q", view)
	}
}

func TestChatClearRemovesTranscript(t *testing.T) {
	t.Parallel()

	model := chat.NewModel()
	for _, key := range []string{"h", "i"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key}))
		model = updated
	}
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Mod: tea.ModCtrl}))
	model = updated

	view := model.View(theme.Default(), "Unified Workspace", 100)
	if strings.Contains(view, "YOU") {
		t.Fatalf("expected cleared transcript, got %q", view)
	}
	if !strings.Contains(view, "Type a question below") {
		t.Fatalf("expected empty state after clear, got %q", view)
	}
}
