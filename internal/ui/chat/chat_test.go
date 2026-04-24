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

// TestChatTraceRendersAtNarrowWidth confirms the trace block degrades
// gracefully on narrow terminals (no panic, title still visible) per the
// UI guide.
func TestChatTraceRendersAtNarrowWidth(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := chat.NewModel(th)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	model = updated
	model = typeKeys(model, "h", "i")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated

	model.AppendStreamTrace(chat.TraceStep{
		Kind:       chat.TraceToolCall,
		ToolCallID: "call_n",
		Title:      "Run SQL",
		Iteration:  1,
	})
	model.AppendStreamText("ok")
	model.CompleteStream()

	view := model.View(th, "Unified Workspace", 40)
	if !strings.Contains(view, "Run SQL") {
		t.Fatalf("expected tool title in narrow render, got %q", view)
	}
}

func TestChatClearTranscriptViaModel(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))
	model = typeKeys(model, "h", "i")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	// Clear is invoked programmatically (logout / workspace switch);
	// there is no longer a ctrl+l keybinding in chat.
	model.Clear()

	view := model.View(th, "Unified Workspace", 120)
	if strings.Contains(view, "YOU") {
		t.Fatalf("expected cleared transcript, got %q", view)
	}
	// Empty state shows the workspace label and textarea placeholder.
	if !strings.Contains(view, "Workspace: Unified Workspace") {
		t.Fatalf("expected workspace label in empty state, got %q", view)
	}
}

// TestChatTraceLifecycle drives a full assistant request: submit, stream
// thoughts and a tool call, then complete. Asserts the live ticker shows
// during streaming, the trace stays visible (expanded) by default after
// the answer arrives, the [shown] badge is rendered, and ctrl+t collapses
// it to a [hidden] summary.
func TestChatTraceLifecycle(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))
	model = typeKeys(model, "h", "i")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated

	// Stream a thought delta and a tool call mid-flight.
	model.MergeStreamThoughtDelta(1, "Inspecting orders table", true)
	model.AppendStreamTrace(chat.TraceStep{
		Kind:       chat.TraceToolCall,
		ToolCallID: "call_1",
		Title:      "Run SQL",
		Iteration:  1,
	})

	view := model.View(th, "Unified Workspace", 120)
	if !strings.Contains(view, "Thinking") {
		t.Fatalf("expected live trace header during stream, got %q", view)
	}
	if !strings.Contains(view, "Inspecting orders table") {
		t.Fatalf("expected thought delta visible during stream, got %q", view)
	}
	if !strings.Contains(view, "Run SQL") {
		t.Fatalf("expected tool call title visible during stream, got %q", view)
	}

	// Finish the stream. Trace stays expanded by default with a [shown]
	// badge so it's obvious that thinking is on display.
	model.AppendStreamText("Done.")
	model.CompleteStream()

	view = model.View(th, "Unified Workspace", 120)
	if !strings.Contains(view, "▾ Thinking") {
		t.Fatalf("expected expanded trace header after complete, got %q", view)
	}
	if !strings.Contains(view, "[shown]") {
		t.Fatalf("expected [shown] badge after complete, got %q", view)
	}
	if !strings.Contains(view, "Run SQL") {
		t.Fatalf("expected tool call still visible after complete, got %q", view)
	}

	// Press ctrl+t to hide. Header switches to collapsed marker + badge.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = updated
	view = model.View(th, "Unified Workspace", 120)
	if !strings.Contains(view, "▸ Thinking") {
		t.Fatalf("expected collapsed trace header after ctrl+t, got %q", view)
	}
	if !strings.Contains(view, "[hidden]") {
		t.Fatalf("expected [hidden] badge after ctrl+t, got %q", view)
	}
	if strings.Contains(view, "Run SQL") {
		t.Fatalf("expected tool call hidden when collapsed, got %q", view)
	}
}

// TestChatToggleAllTraces verifies ctrl+t flips every assistant message's
// trace at once and works while the textarea contains text.
func TestChatToggleAllTraces(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	model := sizeModel(t, chat.NewModel(th))

	// First request with a trace.
	model = typeKeys(model, "a")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	model.AppendStreamTrace(chat.TraceStep{
		Kind: chat.TraceToolCall, ToolCallID: "x1", Title: "Tool A",
	})
	model.AppendStreamText("first")
	model.CompleteStream()

	// Second request with a trace.
	model = typeKeys(model, "b")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated
	model.AppendStreamTrace(chat.TraceStep{
		Kind: chat.TraceToolCall, ToolCallID: "x2", Title: "Tool B",
	})
	model.AppendStreamText("second")
	model.CompleteStream()

	// Both traces start expanded by default.
	view := model.View(th, "Unified Workspace", 120)
	if strings.Count(view, "▾ Thinking") != 2 {
		t.Fatalf("expected two expanded trace headers, got %q", view)
	}
	if !strings.Contains(view, "Tool A") || !strings.Contains(view, "Tool B") {
		t.Fatalf("expected both tool titles visible by default, got %q", view)
	}

	// Type into the textarea to confirm ctrl+t still fires.
	model = typeKeys(model, "c")

	// ctrl+t collapses every trace.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = updated
	view = model.View(th, "Unified Workspace", 120)
	if strings.Count(view, "▸ Thinking") != 2 {
		t.Fatalf("expected two collapsed trace headers after ctrl+t, got %q", view)
	}
	if strings.Contains(view, "Tool A") || strings.Contains(view, "Tool B") {
		t.Fatalf("expected tool titles hidden after collapse, got %q", view)
	}

	// ctrl+t again expands everything.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model = updated
	view = model.View(th, "Unified Workspace", 120)
	if strings.Count(view, "▾ Thinking") != 2 {
		t.Fatalf("expected two expanded trace headers after second ctrl+t, got %q", view)
	}
}
