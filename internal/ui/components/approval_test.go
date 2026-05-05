package components_test

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components"
)

func TestApprovalDialogAllowFocused(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.ApprovalDialog{
		ToolDisplayName:   "Web Search",
		Message:           "Allow web search?",
		ActionDescription: "Search the web for: golang sse",
		SecondsRemaining:  60,
		AllowSelected:     true,
	}.View(th, 100))

	if !strings.Contains(out, "Allow Web Search?") {
		t.Fatalf("expected title with tool name, got %q", out)
	}
	if !strings.Contains(out, "Allow web search?") {
		t.Fatalf("expected backend message, got %q", out)
	}
	if !strings.Contains(out, "Search the web for: golang sse") {
		t.Fatalf("expected action description, got %q", out)
	}
	if !strings.Contains(out, "Auto-deny in 60s") {
		t.Fatalf("expected countdown, got %q", out)
	}
	if !strings.Contains(out, "Allow") || !strings.Contains(out, "Deny") {
		t.Fatalf("expected both buttons, got %q", out)
	}
}

func TestApprovalDialogDenyFocusedHidesCountdownWhenZero(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.ApprovalDialog{
		ToolDisplayName:  "Web Search",
		Message:          "Allow web search?",
		SecondsRemaining: 0,
		AllowSelected:    false,
	}.View(th, 100))

	if strings.Contains(out, "Auto-deny") {
		t.Fatalf("expected no countdown when SecondsRemaining is zero, got %q", out)
	}
}

func TestApprovalDialogFallbackCopy(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := stripANSI(components.ApprovalDialog{
		AllowSelected: true,
	}.View(th, 80))

	if !strings.Contains(out, "Allow this action?") {
		t.Fatalf("expected fallback title, got %q", out)
	}
	if !strings.Contains(out, "The agent wants to run this tool.") {
		t.Fatalf("expected fallback prompt, got %q", out)
	}
}

func TestApprovalDialogNarrowTerminal(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	out := components.ApprovalDialog{
		ToolDisplayName:  "Web Search",
		Message:          "Allow?",
		SecondsRemaining: 5,
		AllowSelected:    true,
	}.View(th, 60)

	if out == "" {
		t.Fatal("expected non-empty render at narrow width")
	}
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "Auto-deny in 5s") {
		t.Fatalf("expected low-time countdown, got %q", stripped)
	}
}
