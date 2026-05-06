package chat

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/teatest"
	"github.com/papermap/papermap-tui/internal/theme"
)

func newTestChat(t *testing.T) Model {
	t.Helper()
	m := NewModel(theme.Default())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func pressKey(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: key}))
	return updated, cmd
}

func TestSetShellModeTogglesAndIdempotent(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	if m.IsShellMode() {
		t.Fatal("default state should not be shell mode")
	}
	m.SetShellMode(true)
	if !m.IsShellMode() {
		t.Fatal("expected shell mode after enable")
	}
	// Idempotent.
	m.SetShellMode(true)
	if !m.IsShellMode() {
		t.Fatal("idempotent enable broke state")
	}
	m.SetShellMode(false)
	if m.IsShellMode() {
		t.Fatal("expected shell mode off after disable")
	}
}

func TestSetShellModeBlockedWhileRunning(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	m.SetShellMode(true)
	m.SetShellRunning(true)
	m.SetShellMode(false)
	if !m.IsShellMode() {
		t.Fatal("disable should be ignored while shell running")
	}
}

func TestEnterInShellModeEmitsShellSubmitMsg(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	m.SetShellMode(true)
	m.textarea.SetValue("ls -la")

	updated, cmd := pressKey(t, m, "enter")
	if !updated.IsShellRunning() {
		t.Fatal("expected shellRunning latched after submit")
	}
	got, ok := teatest.FindMsg[ShellSubmitMsg](cmd)
	if !ok {
		t.Fatalf("expected ShellSubmitMsg in batch")
	}
	if got.Command != "ls -la" {
		t.Fatalf("Command = %q", got.Command)
	}
}

func TestEnterInShellModeRejectsNewlines(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	m.SetShellMode(true)
	m.textarea.SetValue("rm -rf /\necho gotcha")

	updated, cmd := pressKey(t, m, "enter")
	if _, ok := teatest.FindMsg[ShellSubmitMsg](cmd); ok {
		t.Fatal("multi-line shell submit must be rejected")
	}
	if updated.IsShellRunning() {
		t.Fatal("shellRunning must not latch on rejected submit")
	}
}

func TestEnterInShellModeBypassesPasteExpansion(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	// Register a paste so the registry has at least one chip; the
	// shell-mode branch must not call expand() on the raw value.
	token, _ := m.pastes.add("rm -rf /")
	m.SetShellMode(true)
	m.textarea.SetValue("echo " + token)

	_, cmd := pressKey(t, m, "enter")
	got, ok := teatest.FindMsg[ShellSubmitMsg](cmd)
	if !ok {
		t.Fatalf("expected ShellSubmitMsg")
	}
	if strings.Contains(got.Command, "rm -rf") {
		t.Fatalf("paste chip leaked into shell command: %q", got.Command)
	}
	if !strings.Contains(got.Command, token) {
		t.Fatalf("expected literal chip token, got %q", got.Command)
	}
}

func TestBackspaceOnEmptyShellExitsMode(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	m.SetShellMode(true)
	updated, _ := pressKey(t, m, "backspace")
	if updated.IsShellMode() {
		t.Fatal("backspace on empty buffer should exit shell mode")
	}
}

func TestAppendShellResultClearsModeAndAppendsTurn(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	m.SetShellMode(true)
	m.SetShellRunning(true)
	m.AppendShellResult(ShellResult{
		Command:  "echo hi",
		Output:   "hi",
		ExitCode: 0,
		Duration: 5 * time.Millisecond,
		CapBytes: 1024,
	})
	if m.IsShellMode() {
		t.Fatal("shell mode should clear after result")
	}
	if m.IsShellRunning() {
		t.Fatal("shell running should clear after result")
	}
	if m.MessageCount() != 1 {
		t.Fatalf("MessageCount = %d", m.MessageCount())
	}
	if m.messages[0].Shell == nil {
		t.Fatal("appended message missing Shell payload")
	}
}

func TestAppendShellResultScrollsToBottom(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	// Pad the transcript so the viewport overflows and GotoBottom
	// is observably different from the initial position.
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{Role: "user", Content: "filler line"})
	}
	m.refreshAfterMutation()
	m.viewport.GotoTop()
	if m.viewport.AtBottom() {
		t.Fatal("precondition: viewport must not start at bottom")
	}
	// Sticky-scroll respects manual scroll. Reset the latch so the
	// append behaves like a user who stayed pinned at the bottom
	// while their command ran.
	m.userScrolled = false

	m.SetShellMode(true)
	m.SetShellRunning(true)
	m.AppendShellResult(ShellResult{
		Command:  "echo hi",
		Output:   "hi",
		ExitCode: 0,
		Duration: time.Millisecond,
		CapBytes: 1024,
	})
	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport at bottom after AppendShellResult")
	}
}

func TestAppendShellResultRespectsUserScroll(t *testing.T) {
	t.Parallel()
	m := newTestChat(t)
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{Role: "user", Content: "filler line"})
	}
	m.refreshAfterMutation()
	m.viewport.GotoTop()
	// Simulate the user having scrolled up while the command ran.
	m.userScrolled = true

	m.SetShellMode(true)
	m.SetShellRunning(true)
	m.AppendShellResult(ShellResult{
		Command:  "echo hi",
		Output:   "hi",
		ExitCode: 0,
		Duration: time.Millisecond,
		CapBytes: 1024,
	})
	if m.viewport.AtBottom() {
		t.Fatal("sticky-scroll violated: jumped to bottom despite userScrolled")
	}
}

func TestRenderShellBlockSurfacesTruncation(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	out := renderShellBlock(th, 80, &ShellResult{
		Command:   "yes",
		Output:    strings.Repeat("a", 100),
		ExitCode:  0,
		Duration:  time.Millisecond,
		Truncated: true,
		CapBytes:  64,
	})
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected truncation marker, got %q", out)
	}
}

func TestRenderShellBlockHidesNoneOnSuccess(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	out := renderShellBlock(th, 80, &ShellResult{
		Command:  "echo ok",
		Output:   "ok",
		ExitCode: 0,
		Duration: 2 * time.Millisecond,
		CapBytes: 1024,
	})
	if !strings.Contains(out, "ok") {
		t.Fatalf("output missing: %q", out)
	}
	if !strings.Contains(out, "exit 0") {
		t.Fatalf("footer missing exit code: %q", out)
	}
	if strings.Contains(out, "truncated") {
		t.Fatalf("unexpected truncation marker: %q", out)
	}
}

func TestRenderMessageShortCircuitsForShell(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	msg := Message{
		Role:    "shell",
		Content: "should-not-render",
		Error:   "should-not-render-either",
		Shell: &ShellResult{
			Command:  "true",
			Output:   "",
			ExitCode: 0,
			Duration: time.Millisecond,
			CapBytes: 1024,
		},
	}
	out := renderMessage(th, 80, msg, "", "", true)
	if strings.Contains(out, "should-not-render") {
		t.Fatalf("shell short-circuit failed; assistant content leaked: %q", out)
	}
	if !strings.Contains(out, "true") {
		t.Fatalf("expected command in render: %q", out)
	}
}
