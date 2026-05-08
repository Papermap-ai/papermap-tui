// Package app — dialog controller.
//
// This file owns the live state and key handling for the generic
// confirmation modal defined in internal/ui/components/dialog. It
// folds the previously separate quit-confirm and SSE-approval flows
// into one primitive so future confirmations are a single openDialog
// call away.
//
// State model:
//
//   - At most one dialog is in flight at a time. The backend serializes
//     SSE confirmations (one per agent run) and the quit confirm is
//     user-driven, so a single pendingDialog pointer is sufficient.
//   - Dialogs are correlated by an opaque id chosen by the caller. The
//     id flows through the tick loop so a stale tick from a dismissed
//     dialog cannot fire on a fresh one.
//   - Resolution is a caller-supplied func(actionID) (tea.Cmd, bool).
//     The bool is keepOpen: true keeps the dialog visible with the
//     submitting flag flipped on (used by SSE while the POST is in
//     flight); false dismisses the dialog as part of resolution. The
//     callback may be nil for flows that need only local state changes.
package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/ui/components/dialog"
)

// pendingDialog tracks the live state of an in-flight dialog. Construct
// via openDialog; mutated by tick / key handlers; cleared by closeDialog
// or resolveDialog.
type pendingDialog struct {
	correlationID string
	request       dialog.Request
	focusedIdx    int
	secondsLeft   int
	submitting    bool
	// onResult is invoked with the chosen action id when the dialog
	// resolves. Return keepOpen=true to keep the dialog visible with
	// the submitting flag flipped on (used by SSE so the modal stays
	// up while the POST is in flight); return false to dismiss the
	// dialog as part of resolution. May be nil for callers that need
	// only local state changes; nil is treated as keepOpen=false.
	onResult func(actionID string) (tea.Cmd, bool)
}

// dialogTickMsg drives the per-second countdown for the active dialog.
// The msg carries the correlation id so a stale tick from a dismissed
// dialog is dropped without affecting the current one.
type dialogTickMsg struct {
	correlationID string
}

// dialogTickCmd schedules the next per-second countdown tick.
func dialogTickCmd(correlationID string) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return dialogTickMsg{correlationID: correlationID}
	})
}

// openDialog installs the supplied request as the active dialog and
// returns the cmd that primes the countdown loop (nil when the request
// has no timeout). Replaces any existing dialog without notifying the
// previous caller; callers that need overlapping confirmations must
// serialize them externally.
func (m *Model) openDialog(correlationID string, req dialog.Request, onResult func(actionID string) (tea.Cmd, bool)) tea.Cmd {
	focusedIdx := dialog.IndexOfAction(req.Actions, req.DefaultID)
	if focusedIdx < 0 {
		focusedIdx = 0
	}
	m.dialog = &pendingDialog{
		correlationID: correlationID,
		request:       req,
		focusedIdx:    focusedIdx,
		secondsLeft:   req.TimeoutSecs,
		onResult:      onResult,
	}
	if req.TimeoutSecs > 0 {
		return dialogTickCmd(correlationID)
	}
	return nil
}

// closeDialog drops the active dialog without invoking the callback.
// Used by reset paths (logout, session expiry, request teardown) where
// the dialog's outcome is no longer meaningful.
func (m *Model) closeDialog() {
	m.dialog = nil
}

// resolveDialog invokes the resolution callback with the chosen action
// id. When the callback returns keepOpen=true the dialog is kept on
// screen with the submitting flag flipped on; otherwise the dialog is
// cleared. Returns nil when no dialog is active.
func (m *Model) resolveDialog(actionID string) tea.Cmd {
	pd := m.dialog
	if pd == nil {
		return nil
	}
	cb := pd.onResult
	if cb == nil {
		m.dialog = nil
		return nil
	}
	cmd, keepOpen := cb(actionID)
	if keepOpen {
		pd.submitting = true
	} else {
		m.dialog = nil
	}
	return cmd
}

// handleDialogTick advances the countdown by one second and resolves
// the dialog with the timeout action when the budget hits zero. Stale
// ticks (mismatched correlation id, dismissed dialog, in-flight submit)
// are dropped.
func (m Model) handleDialogTick(msg dialogTickMsg) (tea.Model, tea.Cmd) {
	pd := m.dialog
	if pd == nil || pd.submitting || pd.correlationID != msg.correlationID {
		return m, nil
	}

	pd.secondsLeft--
	if pd.secondsLeft <= 0 {
		pd.secondsLeft = 0
		return m, m.resolveDialog(pd.request.TimeoutAct)
	}
	return m, dialogTickCmd(pd.correlationID)
}

// updateDialog handles keypresses while a dialog is in flight. Tab /
// arrows / h / l cycle focus; Enter resolves to the focused action;
// hotkeys resolve directly; Esc resolves to DismissID when set; Ctrl+C
// always falls through to the outer quit handler so a stuck modal can
// be escaped.
func (m Model) updateDialog(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pd := m.dialog
	if pd == nil || pd.submitting {
		return m, nil
	}

	key := msg.String()
	switch key {
	case "left", "shift+tab", "h":
		pd.focusedIdx = wrapIdx(pd.focusedIdx-1, len(pd.request.Actions))
		return m, nil
	case "right", "tab", "l":
		pd.focusedIdx = wrapIdx(pd.focusedIdx+1, len(pd.request.Actions))
		return m, nil
	case keyEnter:
		actions := pd.request.Actions
		if pd.focusedIdx < 0 || pd.focusedIdx >= len(actions) {
			return m, nil
		}
		return m, m.resolveDialog(actions[pd.focusedIdx].ID)
	case keyEscape:
		if id := strings.TrimSpace(pd.request.DismissID); id != "" {
			return m, m.resolveDialog(id)
		}
		return m, nil
	}

	if r := singleRune(key); r != 0 {
		if idx := dialog.ResolveHotkey(pd.request.Actions, r); idx >= 0 {
			return m, m.resolveDialog(pd.request.Actions[idx].ID)
		}
	}
	return m, nil
}

// overlayDialog composites the active dialog over the supplied base
// view. No-op when no dialog is active.
func (m Model) overlayDialog(base string) string {
	pd := m.dialog
	if pd == nil {
		return base
	}
	view := dialog.Dialog{
		Request:          pd.request,
		FocusedIdx:       pd.focusedIdx,
		SecondsRemaining: pd.secondsLeft,
		Submitting:       pd.submitting,
	}.View(m.theme, m.width)
	return m.centerOverlay(base, view)
}

// wrapIdx returns idx wrapped into [0, n). Returns 0 when n <= 0 so
// callers do not have to guard against empty action slices.
func wrapIdx(idx, n int) int {
	if n <= 0 {
		return 0
	}
	idx %= n
	if idx < 0 {
		idx += n
	}
	return idx
}

// singleRune returns the rune when key is a single-character string, or
// 0 otherwise. Used to filter "tab"/"enter"-style names out of hotkey
// dispatch.
func singleRune(key string) rune {
	rs := []rune(key)
	if len(rs) != 1 {
		return 0
	}
	return rs[0]
}
