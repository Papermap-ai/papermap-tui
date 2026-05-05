package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// ApprovalDialog renders a modal that asks the user to allow or deny a
// privileged tool action requested by the agent (e.g. web search). It is
// pure presentation: callers own selection state, the countdown value,
// and key handling.
//
// The dialog mirrors the visual structure of ConfirmDialog so the two
// modals feel like siblings, but adds:
//
//   - A title taken from the tool display name with a leading "Allow"
//     verb so the action verb is consistent across tools.
//   - An action description rendered below the prompt copy.
//   - An "Auto-deny in Ns" countdown line that switches to the error
//     color when the remaining budget drops below ten seconds.
type ApprovalDialog struct {
	// ToolDisplayName is the human-readable tool name surfaced by the
	// backend (e.g. "Web Search"). Empty falls back to "this action".
	ToolDisplayName string
	// Message is the short prompt copy from the backend
	// (`confirmation_required.message`). Empty falls back to a generic
	// "The agent wants to run this tool." line.
	Message string
	// ActionDescription is the longer detail string from the backend
	// (`confirmation_required.action_description`). Optional.
	ActionDescription string
	// SecondsRemaining is the live countdown value. Values <= 0 hide
	// the auto-deny line entirely (used for the no-timeout case).
	SecondsRemaining int
	// AllowSelected reports whether the Allow button is focused.
	AllowSelected bool
}

// View renders the dialog as a bordered panel meant to be composited over
// other content via a lipgloss Layer/Compositor.
func (d ApprovalDialog) View(th theme.Theme, screenWidth int) string {
	tool := strings.TrimSpace(d.ToolDisplayName)
	if tool == "" {
		tool = "this action"
	}
	title := "Allow " + tool + "?"

	prompt := strings.TrimSpace(d.Message)
	if prompt == "" {
		prompt = "The agent wants to run this tool."
	}

	allowSelectedBtn := lipgloss.NewStyle().
		Foreground(th.InputBg).
		Background(th.LogoColorA).
		Bold(true).
		Padding(0, 3)
	denySelectedBtn := lipgloss.NewStyle().
		Foreground(th.TextColor).
		Background(lipgloss.Color("#FF7A7A")).
		Bold(true).
		Padding(0, 3)
	unselectedBtn := lipgloss.NewStyle().
		Foreground(th.TextColor).
		Background(th.ButtonBgInactive).
		Padding(0, 3)

	var allowBtn, denyBtn string
	if d.AllowSelected {
		allowBtn = allowSelectedBtn.Render("Allow")
		denyBtn = unselectedBtn.Render("Deny")
	} else {
		allowBtn = unselectedBtn.Render("Allow")
		denyBtn = denySelectedBtn.Render("Deny")
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, allowBtn, "  ", denyBtn)

	panelWidth := approvalPanelWidth(screenWidth)
	// Inner content width = panel width minus border (2) and horizontal
	// padding (3 each side). Text styles must respect this width so
	// wrapped lines stay centered as a block instead of overflowing
	// and forcing the panel's own Align to fall back to left.
	innerWidth := panelWidth - 2 - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	centerLine := func(style lipgloss.Style, s string) string {
		return style.Width(innerWidth).Align(lipgloss.Center).Render(s)
	}

	lines := []string{centerLine(th.Title, title)}
	lines = append(lines, centerLine(th.Body, prompt))
	if action := strings.TrimSpace(d.ActionDescription); action != "" {
		lines = append(lines, "", centerLine(th.Muted, action))
	}
	if d.SecondsRemaining > 0 {
		countdown := fmt.Sprintf("Auto-deny in %ds", d.SecondsRemaining)
		style := th.Muted
		if d.SecondsRemaining < 10 {
			style = th.Error
		}
		lines = append(lines, "", centerLine(style, countdown))
	}
	lines = append(lines, "", lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, buttons))

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Panel border tracks the focused button so the dialog reads as
	// either accept-leaning (brand green) or deny-leaning (error red).
	border := th.LogoColorA
	if !d.AllowSelected {
		border = lipgloss.Color("#FF7A7A")
	}

	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(1, 3).
		Width(panelWidth)

	return panel.Render(body)
}

// approvalPanelWidth scales the approval dialog with the terminal width.
// The dialog is slightly wider than ConfirmDialog because the action
// description tends to be longer than a yes/no prompt.
func approvalPanelWidth(screenWidth int) int {
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
