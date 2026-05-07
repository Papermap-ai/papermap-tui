// Package dialog provides a generic, action-driven confirmation modal.
//
// The Dialog component is intentionally pure presentation: callers own
// focus, countdown, and submission state, and pass them in as fields on
// the Dialog value. The component renders a bordered panel with a title,
// optional body and detail copy, an optional countdown line, and a row
// of buttons derived from the Action list.
//
// The motivation for a single primitive is reuse: prior to this package,
// the app had two parallel modal stacks - one for tool-call approval
// from the agent SSE stream, one for "are you sure you want to quit?".
// Both followed the same shape (title, copy, focused button, key
// handling) but lived in separate components and separate Update paths.
// Folding them into one removes duplication and makes future
// confirmations (delete conversation, destructive workspace actions,
// new SSE-driven approvals) one short Request away.
package dialog

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// Tone influences a single button's focused colors and the panel border
// when that button is focused. Three tones cover the common cases:
//
//   - ToneAccept reads as "go ahead, this is the happy path" (brand
//     green). Used for Allow / Confirm / Save.
//   - ToneDanger reads as "this is destructive or denies an agent
//     action" (error red). Used for Deny / Delete / Discard.
//   - ToneNeutral reads as "this is a non-committal choice" (muted
//     button background, bold text). Used for Cancel / Nope.
type Tone int

const (
	ToneNeutral Tone = iota
	ToneAccept
	ToneDanger
)

// Action is a single button on the dialog. Actions are rendered
// left-to-right in the order supplied. ID is the opaque value reported
// back to the caller when the action is chosen; it is never displayed.
//
// Hotkey is optional; when set, pressing the key (case-insensitively)
// resolves the dialog to this action. Hotkeys must be unique within a
// Request; duplicates resolve to the first matching action.
type Action struct {
	ID     string
	Label  string
	Tone   Tone
	Hotkey rune
}

// Request is the static description of a confirmation dialog. It is
// constructed by callers and handed to the controller. The controller
// owns mutable state (focused index, countdown, submission) and
// re-renders the Dialog value on every frame from Request + state.
type Request struct {
	// Title is the bold heading at the top of the panel. Optional.
	Title string
	// Body is the primary copy under the title. Optional.
	Body string
	// Detail is secondary copy rendered in the muted style under Body.
	// Use for verbose action descriptions that should not crowd the
	// primary prompt. Optional.
	Detail string
	// Actions are the buttons rendered along the bottom of the panel.
	// At least one action is required for the dialog to be useful;
	// callers passing an empty slice get a render with no buttons,
	// which leaves the dialog effectively undismissable from the UI.
	Actions []Action
	// DefaultID names the action that should start focused. Empty
	// defaults to the first action.
	DefaultID string
	// DismissID names the action that fires when the user presses Esc.
	// Empty disables Esc dismissal so the user must pick a button.
	DismissID string
	// TimeoutSecs is the wall-clock budget the user has to respond.
	// Zero disables the countdown line and the auto-fire behavior.
	TimeoutSecs int
	// TimeoutAct is the action ID that fires when the countdown hits
	// zero. Required when TimeoutSecs > 0; ignored otherwise.
	TimeoutAct string
}

// Dialog is the renderable shape: Request plus the live state the
// controller mutates between frames.
type Dialog struct {
	Request
	// FocusedIdx is the index into Request.Actions of the currently
	// focused button. Out-of-range values clamp to a no-focus render
	// (no button highlighted, neutral border).
	FocusedIdx int
	// SecondsRemaining is the live countdown value. Hidden when <= 0.
	SecondsRemaining int
	// Submitting reports whether a submission is in flight. When true,
	// the dialog renders a "Submitting..." line in place of the
	// countdown to communicate that key input is being ignored.
	Submitting bool
}

// View renders the dialog as a bordered panel meant to be composited
// over other content via a lipgloss Layer/Compositor.
func (d Dialog) View(th theme.Theme, screenWidth int) string {
	panelWidth := panelWidth(screenWidth)
	// Inner content width = panel width minus border (2) and
	// horizontal padding (3 each side). Text styles must respect this
	// width so wrapped lines stay centered as a block instead of
	// overflowing left.
	innerWidth := panelWidth - 2 - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	centerLine := func(style lipgloss.Style, s string) string {
		return style.Width(innerWidth).Align(lipgloss.Center).Render(s)
	}

	var lines []string
	if title := strings.TrimSpace(d.Title); title != "" {
		lines = append(lines, centerLine(th.Title, title))
	}
	if body := strings.TrimSpace(d.Body); body != "" {
		lines = append(lines, centerLine(th.Body, body))
	}
	if detail := strings.TrimSpace(d.Detail); detail != "" {
		lines = append(lines, "", centerLine(th.Muted, detail))
	}

	switch {
	case d.Submitting:
		lines = append(lines, "", centerLine(th.Muted, "Submitting..."))
	case d.TimeoutSecs > 0 && d.SecondsRemaining > 0:
		countdown := fmt.Sprintf("Auto-%s in %ds", autoVerb(d.Request), d.SecondsRemaining)
		style := th.Muted
		if d.SecondsRemaining < 10 {
			style = th.Error
		}
		lines = append(lines, "", centerLine(style, countdown))
	}

	buttons := renderButtons(d, th)
	lines = append(lines, "", lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, buttons))

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	border := borderColor(d, th)

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(1, 3).
		Width(panelWidth)

	return panel.Render(body)
}

// renderButtons builds the horizontal button row. Each button is styled
// according to its Tone and whether it is the focused action.
func renderButtons(d Dialog, th theme.Theme) string {
	if len(d.Actions) == 0 {
		return ""
	}

	parts := make([]string, 0, len(d.Actions)*2-1)
	for i, action := range d.Actions {
		if i > 0 {
			parts = append(parts, "  ")
		}
		focused := i == d.FocusedIdx
		parts = append(parts, renderButton(action, focused, th))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// renderButton picks foreground/background based on tone and focus.
// Focused accept buttons take the brand green, focused danger buttons
// take the error red, focused neutral buttons stay on the inactive
// background but bold up so the focus ring is still visible.
func renderButton(action Action, focused bool, th theme.Theme) string {
	style := lipgloss.NewStyle().Padding(0, 3)
	if !focused {
		return style.
			Foreground(th.TextColor).
			Background(th.ButtonBgInactive).
			Render(action.Label)
	}

	switch action.Tone {
	case ToneAccept:
		return style.
			Foreground(th.InputBg).
			Background(th.LogoColorA).
			Bold(true).
			Render(action.Label)
	case ToneDanger:
		return style.
			Foreground(th.TextColor).
			Background(th.ErrorColor).
			Bold(true).
			Render(action.Label)
	default:
		return style.
			Foreground(th.TextColor).
			Background(th.ButtonBgInactive).
			Bold(true).
			Render(action.Label)
	}
}

// borderColor tracks the focused action's tone so the panel reads as
// either accept-leaning, danger-leaning, or neutral.
func borderColor(d Dialog, th theme.Theme) color.Color {
	if d.FocusedIdx < 0 || d.FocusedIdx >= len(d.Actions) {
		return th.LogoColorA
	}
	switch d.Actions[d.FocusedIdx].Tone {
	case ToneDanger:
		return th.ErrorColor
	case ToneAccept:
		return th.LogoColorA
	default:
		return th.LogoColorA
	}
}

// autoVerb derives the countdown verb from the timeout action's label
// when possible (e.g. "deny" -> "Auto-deny in 12s"). Falls back to a
// generic "dismiss" when no timeout action label is available.
func autoVerb(r Request) string {
	for _, action := range r.Actions {
		if action.ID == r.TimeoutAct {
			label := strings.ToLower(strings.TrimSpace(action.Label))
			if label != "" {
				return label
			}
			break
		}
	}
	return "dismiss"
}

// IndexOfAction returns the index of the action with the given ID, or
// -1 when no match is found. Exposed so controllers can resolve the
// initial focused index from Request.DefaultID.
func IndexOfAction(actions []Action, id string) int {
	for i, action := range actions {
		if action.ID == id {
			return i
		}
	}
	return -1
}

// ResolveHotkey returns the index of the action whose Hotkey matches
// the given rune, or -1 when no action claims it. Match is
// case-insensitive on ASCII letters.
func ResolveHotkey(actions []Action, r rune) int {
	if r == 0 {
		return -1
	}
	target := normalizeHotkey(r)
	for i, action := range actions {
		if action.Hotkey == 0 {
			continue
		}
		if normalizeHotkey(action.Hotkey) == target {
			return i
		}
	}
	return -1
}

func normalizeHotkey(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// panelWidth scales the dialog with the terminal width while keeping
// it visibly compact. The bounds match the prior ApprovalDialog so the
// modal does not shrink when callers migrate from the old component;
// the small ConfirmDialog used to fit at 46-64 cols, but the wider
// approval flow is the dominant case and the extra width is harmless
// for short prompts.
func panelWidth(screenWidth int) int {
	const (
		minW = 52
		maxW = 76
	)
	if screenWidth <= 0 {
		return minW
	}
	width := (screenWidth - 6) / 2
	if width > maxW {
		width = maxW
	}
	if width < minW {
		width = screenWidth - 6
		if width < 32 {
			width = 32
		}
	}
	return width
}
