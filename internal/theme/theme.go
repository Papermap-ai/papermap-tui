package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/ui/components/charts"
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
	// ErrorBadge renders a compact pink "ERROR" pill placed inline in
	// the chat transcript when an assistant turn fails or is cancelled.
	ErrorBadge lipgloss.Style
	Accent     lipgloss.Style
	// Selection is the visual style applied to mouse-dragged transcript
	// highlights: a warm amber background with dark ink text on top so
	// the selected region reads as a contiguous swept-highlighter block
	// regardless of any pre-existing ANSI styling underneath it. When a
	// light theme variant lands, swap the amber for a deeper amber
	// (e.g. #E8A317) with white fg so the band stays readable on a
	// pale background.
	Selection lipgloss.Style
	// InputAccent is a softer accent reserved for the prompt input bar so
	// it reads as distinct from assistant message accents.
	InputAccent lipgloss.Style
	InputBg     color.Color // Distinct background for the text input area.
	// MutedColor and TextColor expose the raw palette entries so callers
	// that style external widgets (textarea, buttons) can match the rest
	// of the UI without re-declaring hex codes.
	MutedColor       color.Color
	TextColor        color.Color
	ButtonBgInactive color.Color
	// SplashLogo is the splash-screen logo color (white) kept distinct
	// from the green brand accent.
	SplashLogo color.Color
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
		ErrorBadge: lipgloss.NewStyle().
			Foreground(text).
			Background(errorColor).
			Bold(true).
			Padding(0, 1),
		Accent: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		Selection: lipgloss.NewStyle().
			Background(lipgloss.Color("#FFD86B")).
			Foreground(inputBg),
		InputAccent: lipgloss.NewStyle().
			Foreground(soft).
			Bold(true),
		LogoColorA:       accent,
		LogoColorB:       soft,
		InputBg:          inputBg,
		MutedColor:       muted,
		TextColor:        text,
		ButtonBgInactive: lipgloss.Color("#2A2A35"),
		SplashLogo:       lipgloss.Color("#FFFFFF"),
	}
}

// ChartPalette projects the theme onto the charts.Palette contract so chart
// renderers stay decoupled from the broader theme surface. The series
// rotation is tuned for terminal contrast against the panel background
// while still echoing the brand accent as the lead color.
func (t Theme) ChartPalette() charts.Palette {
	return charts.Palette{
		Series: []color.Color{
			t.LogoColorA,          // brand green leads.
			t.LogoColorB,          // soft mint complements.
			lipgloss.Color("39"),  // blue.
			lipgloss.Color("214"), // orange.
			lipgloss.Color("213"), // pink.
			lipgloss.Color("226"), // yellow.
			lipgloss.Color("105"), // purple.
		},
		Axis:  t.MutedColor,
		Grid:  lipgloss.Color("238"),
		Label: t.TextColor,
		Muted: t.Muted,
	}
}
