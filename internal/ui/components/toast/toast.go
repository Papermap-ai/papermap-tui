// Package toast provides a small ephemeral status overlay used to
// confirm transient actions (copy, save, etc.) without stealing focus.
//
// A toast is a presentational sub-model: callers store a Model on their
// root model, drive it with Show / Update, and place its rendered View
// onto the existing layer (typically pinned to a corner). Dismissal is
// driven by a tea.Cmd that fires after the configured duration; no
// global timers, no background goroutines.
package toast

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// DefaultDuration is used when Show is called with a non-positive
// duration. 1.5 seconds is short enough to feel responsive but long
// enough for a glance, matching the original product spec.
const DefaultDuration = 1500 * time.Millisecond

// Kind selects a visual variant. Success uses the brand accent;
// Info uses a muted style. Failure variants can be added later.
type Kind int

const (
	KindSuccess Kind = iota
	KindInfo
)

// dismissMsg fires after the configured duration. The token field
// guards against a stale timer dismissing a newer toast: each Show
// bumps the token, and Update only honors a dismiss whose token
// matches the current toast.
type dismissMsg struct {
	token int
}

// Model holds the visible toast (if any) and the dismissal token used
// to ignore stale timers. Zero value is valid and renders empty.
type Model struct {
	text  string
	kind  Kind
	token int
	live  bool
}

// New returns an empty toast model. Provided for symmetry with other
// Bubble Tea models; the zero value works just as well.
func New() Model {
	return Model{}
}

// Show replaces the current toast with a new one and returns a Cmd
// that will dismiss it after the given duration. Pass 0 to use
// DefaultDuration. The returned Cmd MUST be returned from the parent
// Update so Bubble Tea schedules the dismissal tick.
func (m *Model) Show(text string, kind Kind, duration time.Duration) tea.Cmd {
	if duration <= 0 {
		duration = DefaultDuration
	}
	m.text = text
	m.kind = kind
	m.live = true
	m.token++
	token := m.token
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return dismissMsg{token: token}
	})
}

// Update handles dismiss ticks. Returns true when the toast was
// dismissed by this message so callers can trigger a redraw if needed.
// All other messages pass through unchanged.
func (m *Model) Update(msg tea.Msg) bool {
	dm, ok := msg.(dismissMsg)
	if !ok {
		return false
	}
	if !m.live || dm.token != m.token {
		return false
	}
	m.live = false
	m.text = ""
	return true
}

// Visible reports whether a toast is currently being displayed.
func (m Model) Visible() bool {
	return m.live && m.text != ""
}

// View renders the toast as a compact pill. Returns "" when no toast
// is live so callers can unconditionally embed the result without
// reserving layout space.
func (m Model) View(th theme.Theme) string {
	if !m.Visible() {
		return ""
	}
	style := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 2).
		Background(th.InputBg).
		Foreground(th.TextColor)
	switch m.kind {
	case KindSuccess:
		style = style.Foreground(th.LogoColorA)
	case KindInfo:
		style = style.Foreground(th.MutedColor)
	}
	return style.Render(m.text)
}
