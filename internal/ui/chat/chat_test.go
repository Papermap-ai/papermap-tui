package chat_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/teatest"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

// sizeModel sends a WindowSizeMsg so the viewport has dimensions and the
// transcript actually renders into it during View().
func sizeModel(t *testing.T, model chat.Model) chat.Model {
	t.Helper()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated
}

func typeKeys(model chat.Model, keys ...string) chat.Model {
	for _, key := range keys {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key}))
		model = updated
	}
	return model
}

func TestChatSubmitStartsStreamingTranscript(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))
	model = typeKeys(model, "h", "i")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	// Enter triggers a batch (SubmitMsg + spinner.Tick); pluck the SubmitMsg.
	submit, ok := teatest.FindMsg[chat.SubmitMsg](cmd)
	if !ok {
		t.Fatal("expected chat.SubmitMsg in batch")
	}
	if submit.Prompt != "hi" {
		t.Fatalf("expected prompt hi, got %q", submit.Prompt)
	}

	view := model.View(th, "Unified Workspace", 120)
	// User message renders with uppercased role; assistant slot is pending
	// and shows the "Thinking..." placeholder until streaming text arrives.
	if !strings.Contains(view, "YOU") {
		t.Fatalf("expected user message in transcript, got %q", view)
	}
	if !strings.Contains(view, "Thinking") {
		t.Fatalf("expected pending assistant placeholder, got %q", view)
	}

	model.AppendStreamText("hello")
	model.CompleteStream()
	view = model.View(th, "Unified Workspace", 120)
	if !strings.Contains(view, "hello") {
		t.Fatalf("expected streamed content in view, got %q", view)
	}
}

func TestChatClearRemovesTranscript(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))
	model = typeKeys(model, "h", "i")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Mod: tea.ModCtrl}))
	model = updated

	view := model.View(th, "Unified Workspace", 120)
	if strings.Contains(view, "YOU") {
		t.Fatalf("expected cleared transcript, got %q", view)
	}
	// Empty state shows the workspace label and textarea placeholder.
	if !strings.Contains(view, "Workspace: Unified Workspace") {
		t.Fatalf("expected workspace label in empty state, got %q", view)
	}
}
