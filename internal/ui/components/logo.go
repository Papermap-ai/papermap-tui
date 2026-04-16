package components

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// Logo renders the full block-art PAPERMAP wordmark with two-tone coloring,
// centered within the given width. "PAPER" uses LogoColorA, "MAP" uses
// LogoColorB.
func Logo(th theme.Theme, width int) string {
	return Render(th, width)
}

// Render renders the block-art PAPERMAP wordmark. If width > 0, the logo
// is centered within that width.
func Render(th theme.Theme, width int) string {
	const spacing = 1

	paperLetters := []string{
		letterP(),
		letterA(),
		letterP(),
		letterE(),
		letterR(),
	}
	mapLetters := []string{
		letterM(),
		letterA(),
		letterP(),
	}

	paper := renderWord(spacing, paperLetters...)
	mapWord := renderWord(spacing, mapLetters...)

	paper = colorize(paper, th.LogoColorA)
	mapWord = colorize(mapWord, th.LogoColorB)

	// Join PAPER and MAP with a small gap.
	logo := lipgloss.JoinHorizontal(
		lipgloss.Top,
		paper,
		" ",
		mapWord,
	)

	if width > 0 {
		logo = lipgloss.PlaceHorizontal(width, lipgloss.Center, logo)
	}

	return logo
}

// SmallRender renders a compact inline "Papermap" with two-tone coloring,
// suitable for narrow terminals. Fills remaining width with a decorative
// line character.
func SmallRender(th theme.Theme, width int) string {
	paperStyle := lipgloss.NewStyle().Bold(true).Foreground(th.LogoColorA)
	mapStyle := lipgloss.NewStyle().Bold(true).Foreground(th.LogoColorB)

	title := paperStyle.Render("Paper") + mapStyle.Render("map")
	titleWidth := lipgloss.Width(title)

	remaining := width - titleWidth - 1 // 1 for space.
	if remaining > 0 {
		line := strings.Repeat("╱", remaining)
		title = fmt.Sprintf("%s %s", title, lipgloss.NewStyle().Foreground(th.LogoColorB).Render(line))
	}

	return title
}

// colorize applies a foreground color to every line of a multi-line string.
func colorize(s string, c color.Color) string {
	style := lipgloss.NewStyle().Foreground(c).Bold(true)
	b := new(strings.Builder)
	first := true
	for line := range strings.SplitSeq(s, "\n") {
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(style.Render(line))
		first = false
	}
	return b.String()
}

// renderWord joins letterform strings horizontally with the given spacing.
func renderWord(spacing int, letters ...string) string {
	if len(letters) == 0 {
		return ""
	}
	if spacing < 1 {
		spacing = 1
	}
	spaced := make([]string, 0, len(letters)*2-1)
	for i, l := range letters {
		if i > 0 {
			spaced = append(spaced, strings.Repeat(" ", spacing))
		}
		spaced = append(spaced, l)
	}
	return strings.TrimRight(
		lipgloss.JoinHorizontal(lipgloss.Top, spaced...),
		" \n",
	)
}

// --- Letterforms ---
// Each letterform is 3 rows tall using Unicode block characters.
// Lowercase style: rounded tops (▄) for letters without ascenders,
// p's stem continues to row 3 as a descender.
//   █ = full block, ▄ = lower half, ▀ = upper half

func letterP() string {
	// █▀▀▄
	// █▄▄▀
	// █
	return join(
		"█\n█\n█",
		"▀▀\n▄▄\n  ",
		"▄\n▀\n ",
	)
}

func letterA() string {
	// ▄▀▀▄
	// █▀▀█
	// ▀  ▀
	return join(
		"▄\n█\n▀",
		"▀▀\n▀▀\n  ",
		"▄\n█\n▀",
	)
}

func letterE() string {
	// ▄▀▀▄
	// █▀▀
	// ▀▀▀
	return join(
		"▄\n█\n▀",
		"▀▀\n▀▀\n▀▀",
		"▄\n \n ",
	)
}

func letterR() string {
	// █▀▀▄
	// █
	// ▀
	return join(
		"█\n█\n▀",
		"▀▀\n  \n  ",
		"▄\n \n ",
	)
}

func letterM() string {
	// ▄▄ ▄▄
	// █▀█▀█
	// ▀ ▀ ▀
	return join(
		"▄\n█\n▀",
		"▄\n▀\n ",
		" \n█\n▀",
		"▄\n▀\n ",
		"▄\n█\n▀",
	)
}

// join horizontally aligns multi-line letter parts.
func join(parts ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
