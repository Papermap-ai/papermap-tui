package app_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/app"
)

// TestSlashOpensPaletteWhenTextareaEmpty verifies that pressing "/" on a
// chat screen with an empty textarea opens the command palette overlay
// rather than being typed as a literal character.
func TestSlashOpensPaletteWhenTextareaEmpty(t *testing.T) {
	t.Parallel()

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SeedChatForTest(120, 40).SetAuthenticatedForTest()

	updated, _ := model.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	got := updated.(app.Model).ScreenName()
	if got != app.ScreenCommandPalette {
		t.Fatalf("expected screen %q after /, got %q", app.ScreenCommandPalette, got)
	}
}

// TestEscapeDismissesPalette verifies that Esc on the palette returns
// the user to the chat screen.
func TestEscapeDismissesPalette(t *testing.T) {
	t.Parallel()

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SeedChatForTest(120, 40).SetAuthenticatedForTest()

	opened, _ := model.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	openedModel := opened.(app.Model)
	if openedModel.ScreenName() != app.ScreenCommandPalette {
		t.Fatalf("expected palette to open, got %q", openedModel.ScreenName())
	}

	// Drive the palette's CancelMsg by sending Esc; the palette's Update
	// returns a CancelMsg cmd which the app consumes to flip back to
	// chat.
	closed, cmd := openedModel.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatalf("expected CancelMsg cmd from palette esc")
	}
	cancelMsg := cmd()
	final, _ := closed.(app.Model).Update(cancelMsg)
	if got := final.(app.Model).ScreenName(); got != app.ScreenChat {
		t.Fatalf("expected screen chat after esc, got %q", got)
	}
}
