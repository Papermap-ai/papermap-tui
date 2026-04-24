package app

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// newSplashSpinner creates a spinner.Model configured for the splash screen.
// It uses spinner.Dot with the theme accent color so the loading indicator
// matches the Papermap brand.
func newSplashSpinner(th theme.Theme) spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.LogoColorA)
	return s
}

// splashView renders the splash screen shown during startup while the
// application restores credentials and loads session data. It displays
// the Papermap logo with a spinner beside it, both horizontally centered
// on the page.
func (m Model) splashView() string {
	logoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.SplashLogo)
	logo := logoStyle.Render("Papermap")
	line := logo + " " + m.spinner.View()
	return line
}
