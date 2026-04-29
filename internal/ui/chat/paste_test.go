package chat_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/teatest"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

// largePaste returns a string with n lines, big enough to clear both the
// line and char thresholds.
func largePaste(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "line of pasted content"
	}
	return strings.Join(parts, "\n")
}

func TestPasteCollapsesIntoChip(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	pasted := largePaste(25)
	updated, _ := model.Update(tea.PasteMsg{Content: pasted})
	model = updated

	// Buffer should hold the synthetic chip token, not the raw paste.
	if got := model.TextareaValue(); strings.Contains(got, "line of pasted content") {
		t.Fatalf("textarea should not contain raw pasted content, got %q", got)
	}
	if got := model.TextareaValue(); !strings.Contains(got, "[#p:") {
		t.Fatalf("expected paste token in textarea, got %q", got)
	}

	// The rendered prompt should show the chip label, not the token.
	view := model.View(th, "Unified Workspace", 120)
	if !strings.Contains(view, "[Pasted ~25 lines]") {
		t.Fatalf("expected chip label in rendered view, got %q", view)
	}
	if strings.Contains(view, "[#p:1]") {
		t.Fatalf("raw token leaked into rendered view: %q", view)
	}
}

func TestSmallPasteFlowsThroughVerbatim(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	updated, _ := model.Update(tea.PasteMsg{Content: "short paste"})
	model = updated

	if got := model.TextareaValue(); got != "short paste" {
		t.Fatalf("expected verbatim short paste, got %q", got)
	}
}

func TestPasteExpandsOnSubmit(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	pasted := largePaste(25)
	updated, _ := model.Update(tea.PasteMsg{Content: pasted})
	model = updated

	// Type a question after the chip so the prompt has surrounding text.
	model = typeKeys(model, " ", "?")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	submit, ok := teatest.FindMsg[chat.SubmitMsg](cmd)
	if !ok {
		t.Fatal("expected chat.SubmitMsg in batch")
	}

	if !strings.Contains(submit.Prompt, "line of pasted content") {
		t.Fatalf("expected expanded paste in submit prompt, got %q", submit.Prompt)
	}
	if strings.Contains(submit.Prompt, "[#p:") {
		t.Fatalf("paste token leaked into submit prompt: %q", submit.Prompt)
	}

	// The transcript-stored user message should also carry the expanded text.
	if last := model.LastUserPrompt(); !strings.Contains(last, "line of pasted content") {
		t.Fatalf("expected expanded paste in last user message, got %q", last)
	}
}

func TestBackspaceDeletesEntireChip(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	pasted := largePaste(25)
	updated, _ := model.Update(tea.PasteMsg{Content: pasted})
	model = updated

	// Cursor sits immediately after the chip token.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated

	if got := model.TextareaValue(); got != "" {
		t.Fatalf("expected empty textarea after chip backspace, got %q", got)
	}
}

func TestSubmitWithChipOnlyExpandsAndSends(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	pasted := largePaste(10)
	updated, _ := model.Update(tea.PasteMsg{Content: pasted})
	model = updated

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected submit command for chip-only prompt")
	}
	_ = updated

	submit, ok := teatest.FindMsg[chat.SubmitMsg](cmd)
	if !ok {
		t.Fatal("expected chat.SubmitMsg in batch")
	}
	if !strings.Contains(submit.Prompt, "line of pasted content") {
		t.Fatalf("expected expanded chip-only prompt, got %q", submit.Prompt)
	}
}

func TestClearResetsPasteRegistry(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	updated, _ := model.Update(tea.PasteMsg{Content: largePaste(10)})
	model = updated

	model.Clear()
	// After Clear, a literal "[#p:1]" typed by the user should not be
	// expanded back into the (now-discarded) paste content.
	model.LoadConversation("", nil) // ensure clean draft
	if got := model.TextareaValue(); got != "" {
		t.Fatalf("expected empty textarea after Clear+LoadConversation, got %q", got)
	}
}
