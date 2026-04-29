package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/app"
)

// TestPasteMsgRoutedToChat verifies the app forwards bracketed-paste
// content to the chat model. Without explicit routing the top-level
// Update switch swallows tea.PasteMsg and large pastes never reach the
// textarea or the chip handler.
func TestPasteMsgRoutedToChat(t *testing.T) {
	t.Parallel()

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SeedChatForTest(120, 40).SetAuthenticatedForTest()

	short := "small paste content"
	updated, _ := model.Update(tea.PasteMsg{Content: short})
	got := updated.(app.Model).Chat().TextareaValue()
	if got != short {
		t.Fatalf("expected textarea to contain pasted text %q, got %q", short, got)
	}
}

// TestLargePasteRoutedAndCollapsed verifies the end-to-end path: the
// app forwards a paste big enough to trip the chip threshold, the chat
// model collapses it, and the rendered prompt shows the chip label.
func TestLargePasteRoutedAndCollapsed(t *testing.T) {
	t.Parallel()

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SeedChatForTest(120, 40).SetAuthenticatedForTest()

	pasted := strings.Repeat("a line of pasted content\n", 25)
	updated, _ := model.Update(tea.PasteMsg{Content: pasted})
	chatModel := updated.(app.Model).Chat()

	if val := chatModel.TextareaValue(); strings.Contains(val, "a line of pasted content") {
		t.Fatalf("expected raw paste to be replaced with chip token, got %q", val)
	}
	if val := chatModel.TextareaValue(); !strings.Contains(val, "[#p:") {
		t.Fatalf("expected chip token in textarea, got %q", val)
	}
}
