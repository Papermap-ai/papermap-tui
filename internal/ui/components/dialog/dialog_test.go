package dialog_test

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components/dialog"
)

// stripANSI removes ANSI escape sequences so assertions can target the
// rendered text content without coupling to specific style codes.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == 0x1b {
			inEscape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func quitRequest() dialog.Request {
	return dialog.Request{
		Title: "Quit Papermap?",
		Body:  "Press y to confirm or n to stay.",
		Actions: []dialog.Action{
			{ID: "no", Label: "Nope", Tone: dialog.ToneNeutral, Hotkey: 'n'},
			{ID: "yes", Label: "Yes, quit", Tone: dialog.ToneDanger, Hotkey: 'y'},
		},
		DefaultID: "no",
		DismissID: "no",
	}
}

func approvalRequest() dialog.Request {
	return dialog.Request{
		Title:  "Allow tool call?",
		Body:   "The agent wants to run a tool.",
		Detail: "list_dashboards",
		Actions: []dialog.Action{
			{ID: "allow", Label: "Allow", Tone: dialog.ToneAccept, Hotkey: 'a'},
			{ID: "deny", Label: "Deny", Tone: dialog.ToneDanger, Hotkey: 'd'},
		},
		DefaultID:   "allow",
		DismissID:   "deny",
		TimeoutSecs: 60,
		TimeoutAct:  "deny",
	}
}

func TestDialogRendersTitleBodyAndButtons(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{Request: quitRequest(), FocusedIdx: 0}

	out := stripANSI(d.View(th, 120))

	for _, want := range []string{"Quit Papermap?", "Press y to confirm", "Nope", "Yes, quit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestDialogRendersDetailWhenPresent(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{Request: approvalRequest(), FocusedIdx: 0, SecondsRemaining: 60}

	out := stripANSI(d.View(th, 120))
	if !strings.Contains(out, "list_dashboards") {
		t.Fatalf("expected detail copy in output, got %q", out)
	}
}

func TestDialogCountdownHiddenWhenZero(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{Request: approvalRequest(), FocusedIdx: 0, SecondsRemaining: 0}

	out := stripANSI(d.View(th, 120))
	if strings.Contains(out, "Auto-") {
		t.Fatalf("expected no countdown line at 0s, got %q", out)
	}
}

func TestDialogCountdownDerivesVerbFromTimeoutAction(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{Request: approvalRequest(), FocusedIdx: 0, SecondsRemaining: 42}

	out := stripANSI(d.View(th, 120))
	if !strings.Contains(out, "Auto-deny in 42s") {
		t.Fatalf("expected auto-deny countdown, got %q", out)
	}
}

func TestDialogSubmittingReplacesCountdown(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{
		Request:          approvalRequest(),
		FocusedIdx:       0,
		SecondsRemaining: 42,
		Submitting:       true,
	}

	out := stripANSI(d.View(th, 120))
	if !strings.Contains(out, "Submitting...") {
		t.Fatalf("expected submitting line, got %q", out)
	}
	if strings.Contains(out, "Auto-") {
		t.Fatalf("expected countdown hidden while submitting, got %q", out)
	}
}

func TestDialogNarrowTerminalStillRenders(t *testing.T) {
	t.Parallel()

	th := theme.Default()
	d := dialog.Dialog{Request: approvalRequest(), FocusedIdx: 1, SecondsRemaining: 5}

	out := d.View(th, 40)
	if out == "" {
		t.Fatal("expected non-empty render at narrow width")
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "Allow") || !strings.Contains(plain, "Deny") {
		t.Fatalf("expected both buttons even at narrow width, got %q", plain)
	}
}

func TestIndexOfActionFindsAndMisses(t *testing.T) {
	t.Parallel()

	actions := approvalRequest().Actions
	if got := dialog.IndexOfAction(actions, "deny"); got != 1 {
		t.Fatalf("expected deny at index 1, got %d", got)
	}
	if got := dialog.IndexOfAction(actions, "missing"); got != -1 {
		t.Fatalf("expected -1 for missing id, got %d", got)
	}
}

func TestResolveHotkeyCaseInsensitive(t *testing.T) {
	t.Parallel()

	actions := approvalRequest().Actions
	if got := dialog.ResolveHotkey(actions, 'A'); got != 0 {
		t.Fatalf("expected uppercase A to resolve to allow, got %d", got)
	}
	if got := dialog.ResolveHotkey(actions, 'd'); got != 1 {
		t.Fatalf("expected lowercase d to resolve to deny, got %d", got)
	}
	if got := dialog.ResolveHotkey(actions, 0); got != -1 {
		t.Fatalf("expected zero rune to be unresolved, got %d", got)
	}
	if got := dialog.ResolveHotkey(actions, 'z'); got != -1 {
		t.Fatalf("expected unknown rune to be unresolved, got %d", got)
	}
}

func TestResolveHotkeyHonorsFirstMatchOnDuplicate(t *testing.T) {
	t.Parallel()

	actions := []dialog.Action{
		{ID: "first", Label: "First", Hotkey: 'x'},
		{ID: "second", Label: "Second", Hotkey: 'x'},
	}
	if got := dialog.ResolveHotkey(actions, 'x'); got != 0 {
		t.Fatalf("expected first match to win, got %d", got)
	}
}
