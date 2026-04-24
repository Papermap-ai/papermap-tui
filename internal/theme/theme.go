package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	App        lipgloss.Style
	Logo       lipgloss.Style
	LogoColorA color.Color // Primary logo color ("PAPER").
	LogoColorB color.Color // Secondary logo color ("MAP").
	Panel      lipgloss.Style
	Title      lipgloss.Style
	Body       lipgloss.Style
	Muted      lipgloss.Style
	KeyHint    lipgloss.Style
	Error      lipgloss.Style
	Accent     lipgloss.Style
	// InputAccent is a softer accent reserved for the prompt input bar so
	// it reads as distinct from assistant message accents.
	InputAccent lipgloss.Style
	InputBg     color.Color // Distinct background for the text input area.
}

func Default() Theme {
	accent := lipgloss.Color("#2ED8A3")
	soft := lipgloss.Color("#7BE7C5")
	muted := lipgloss.Color("#97A6A8")
	text := lipgloss.Color("#F2F5F4")
	border := lipgloss.Color("#23403D")
	errorColor := lipgloss.Color("#FF7A7A")
	inputBg := lipgloss.Color("#11111B")

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
		KeyHint: lipgloss.NewStyle().
			Foreground(muted),
		Error: lipgloss.NewStyle().
			Foreground(errorColor),
		Accent: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		InputAccent: lipgloss.NewStyle().
			Foreground(soft).
			Bold(true),
		LogoColorA: accent,
		LogoColorB: soft,
		InputBg:    inputBg,
	}
}
