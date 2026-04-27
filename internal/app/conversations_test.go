package app

import (
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// TestChatMessagesFromConversationsBuildsBackfillTargets verifies that
// the conversion produces (user, assistant) message pairs and emits a
// backfill target whose messageIndex points at the assistant message
// for every entry that carries an llm_data_id. Targets must be omitted
// for entries with no id so we never dispatch a doomed GetChart call.
func TestChatMessagesFromConversationsBuildsBackfillTargets(t *testing.T) {
	t.Parallel()

	entries := []api.ConversationEntry{
		{
			LLMDataID:    "id-1",
			UserQuery:    "first question",
			TextResponse: "first answer",
		},
		{
			// Entry with no llm_data_id should not produce a target.
			UserQuery:    "second question",
			TextResponse: "second answer",
		},
		{
			LLMDataID:    "id-3",
			UserQuery:    "", // No user message -> assistant-only turn.
			TextResponse: "rundown summary",
			Thoughts:     "thinking trace",
		},
	}

	messages, targets := chatMessagesFromConversations(entries)

	// Entry 1 contributes (user, assistant) -> indices 0, 1.
	// Entry 2 contributes (user, assistant) -> indices 2, 3.
	// Entry 3 contributes (assistant)       -> index   4.
	if got, want := len(messages), 5; got != want {
		t.Fatalf("message count: got %d, want %d", got, want)
	}
	if messages[1].Role != "alan" || messages[1].Content != "first answer" {
		t.Fatalf("messages[1] mismatch: %+v", messages[1])
	}
	if messages[3].Role != "alan" || messages[3].Content != "second answer" {
		t.Fatalf("messages[3] mismatch: %+v", messages[3])
	}
	if messages[4].Role != "alan" || messages[4].Content != "rundown summary" {
		t.Fatalf("messages[4] mismatch: %+v", messages[4])
	}
	if !messages[4].TraceComplete || len(messages[4].Trace) != 1 {
		t.Fatalf("expected completed thoughts trace on messages[4], got %+v", messages[4])
	}

	if got, want := len(targets), 2; got != want {
		t.Fatalf("target count: got %d, want %d", got, want)
	}
	if targets[0].llmDataID != "id-1" || targets[0].messageIndex != 1 {
		t.Fatalf("targets[0] mismatch: %+v", targets[0])
	}
	if targets[1].llmDataID != "id-3" || targets[1].messageIndex != 4 {
		t.Fatalf("targets[1] mismatch: %+v", targets[1])
	}
}

// TestChatMessagesFromConversationsEmpty verifies the nil-in / nil-out
// contract so callers can dispatch the result of fetchChartBackfills
// without a length guard.
func TestChatMessagesFromConversationsEmpty(t *testing.T) {
	t.Parallel()

	messages, targets := chatMessagesFromConversations(nil)
	if messages != nil {
		t.Fatalf("expected nil messages, got %+v", messages)
	}
	if targets != nil {
		t.Fatalf("expected nil targets, got %+v", targets)
	}
}
