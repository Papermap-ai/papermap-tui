//go:build unix

package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/app"
	"github.com/papermap/papermap-tui/internal/teatest"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

func bangKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: "!"})
}

func TestBangIntercepted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)

	updated, _ := model.Update(bangKey())
	updatedApp, ok := updated.(app.Model)
	if !ok {
		t.Fatalf("expected app.Model, got %T", updated)
	}
	if !updatedApp.Chat().IsShellMode() {
		t.Fatal("! should latch shell mode when textarea empty")
	}
	if !updatedApp.Chat().TextareaIsEmpty() {
		t.Fatal("! should not be inserted into the textarea")
	}
}

func TestBangNotInterceptedWhileTyping(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)
	// Inject a non-empty textarea.
	chatModel := model.Chat()
	chatModel, _ = chatModel.Update(tea.KeyPressMsg(tea.Key{Text: "h"}))
	model = model.WithChatForTest(chatModel)

	updated, _ := model.Update(bangKey())
	updatedApp := updated.(app.Model)
	if updatedApp.Chat().IsShellMode() {
		t.Fatal("! must not latch shell mode mid-prompt")
	}
}

func TestShellResultRoundTripAppendsTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)

	// Enter shell mode and submit.
	updated, _ := model.Update(bangKey())
	model = updated.(app.Model)
	chatModel := model.Chat()
	chatModel.TextareaSetValueForTest("printf hello")
	model = model.WithChatForTest(chatModel)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(app.Model)
	if !model.Chat().IsShellRunning() {
		t.Fatal("expected shellRunning after enter")
	}
	if _, ok := teatest.FindMsg[chat.ShellSubmitMsg](cmd); !ok {
		t.Fatal("expected ShellSubmitMsg from chat enter")
	}

	// Drive the ShellSubmitMsg through the app to start the worker.
	updated, cmd = model.Update(chat.ShellSubmitMsg{Command: "printf hello"})
	model = updated.(app.Model)
	if cmd == nil {
		t.Fatal("expected start command from app")
	}

	// Drain the worker and feed the resulting message back to Update.
	deadline := time.After(3 * time.Second)
	resultCh := make(chan tea.Msg, 1)
	go func() { resultCh <- cmd() }()
	var resultMsg tea.Msg
	select {
	case resultMsg = <-resultCh:
	case <-deadline:
		t.Fatal("shell worker timed out")
	}

	updated, _ = model.Update(resultMsg)
	model = updated.(app.Model)

	if model.Chat().IsShellRunning() {
		t.Fatal("shellRunning should clear after result")
	}
	if model.Chat().IsShellMode() {
		t.Fatal("shellMode should clear after result")
	}
	if !strings.Contains(model.Chat().TranscriptForTest(80), "hello") {
		t.Fatalf("expected hello in transcript: %q", model.Chat().TranscriptForTest(80))
	}
	if !strings.Contains(model.Chat().TranscriptForTest(80), "printf hello") {
		t.Fatalf("expected command echo in transcript")
	}
}
