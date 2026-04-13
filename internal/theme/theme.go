package theme

import "charm.land/lipgloss/v2"

type Theme struct {
	App     lipgloss.Style
	Logo    lipgloss.Style
	Panel   lipgloss.Style
	Title   lipgloss.Style
	Body    lipgloss.Style
	Muted   lipgloss.Style
	Status  lipgloss.Style
	KeyHint lipgloss.Style
	Error   lipgloss.Style
	Accent  lipgloss.Style
}

func Default() Theme {
	accent := lipgloss.Color("#8B5CF6")
	soft := lipgloss.Color("#A78BFA")
	muted := lipgloss.Color("#94A3B8")
	text := lipgloss.Color("#E2E8F0")
	border := lipgloss.Color("#334155")
	errorColor := lipgloss.Color("#F87171")

	return Theme{
		App: lipgloss.NewStyle().
			Foreground(text).
			Padding(1, 2),
		Logo: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		Panel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(text),
		Body: lipgloss.NewStyle().
			Foreground(text),
		Muted: lipgloss.NewStyle().
			Foreground(muted),
		Status: lipgloss.NewStyle().
			Foreground(soft),
		KeyHint: lipgloss.NewStyle().
			Foreground(muted),
		Error: lipgloss.NewStyle().
			Foreground(errorColor),
		Accent: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
	}
}
