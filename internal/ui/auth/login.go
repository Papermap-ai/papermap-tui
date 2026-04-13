package auth

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

type focusField int

const (
	focusEmail focusField = iota
	focusPassword
)

type SubmitMsg struct {
	Email    string
	Password string
}

type Model struct {
	email        string
	password     string
	focus        focusField
	isSubmitting bool
	err          string
}

func NewModel() Model {
	return Model{}
}

func (m Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.isSubmitting {
		return m, nil
	}

	switch msg.String() {
	case "tab", "shift+tab", "up", "down":
		m.focus = m.nextFocus(msg.String())
		return m, nil
	case "backspace":
		m.deleteRune()
		return m, nil
	case "enter":
		m.err = ""
		if strings.TrimSpace(m.email) == "" {
			m.focus = focusEmail
			m.err = "Enter your email to continue."
			return m, nil
		}
		if m.password == "" {
			m.focus = focusPassword
			m.err = "Enter your password to continue."
			return m, nil
		}

		m.isSubmitting = true
		return m, func() tea.Msg {
			return SubmitMsg{
				Email:    strings.TrimSpace(m.email),
				Password: m.password,
			}
		}
	}

	if msg.Key().Text != "" {
		m.err = ""
		m.insertText(msg.Key().Text)
	}

	return m, nil
}

func (m *Model) SetSubmitting(submitting bool) {
	m.isSubmitting = submitting
}

func (m *Model) SetError(err string) {
	m.err = strings.TrimSpace(err)
	m.isSubmitting = false
}

func (m *Model) Reset() {
	m.email = ""
	m.password = ""
	m.focus = focusEmail
	m.isSubmitting = false
	m.err = ""
}

func (m Model) View(th theme.Theme, width int) string {
	panelWidth := clampWidth(width, 64)
	content := []string{
		th.Title.Render("Sign in"),
		"",
		th.Muted.Render("Use your Papermap account to continue."),
		"",
		fieldView(th, "Email", m.email, m.focus == focusEmail, false),
		"",
		fieldView(th, "Password", masked(m.password), m.focus == focusPassword, true),
	}

	if m.err != "" {
		content = append(content, "", th.Error.Render(m.err))
	}

	if m.isSubmitting {
		content = append(content, "", th.Accent.Render("Signing in..."))
	} else {
		content = append(content, "", th.Accent.Render("Enter to sign in"))
	}

	content = append(content, "", th.KeyHint.Render("Tab switch fields  •  Enter submit  •  Esc back"))

	return th.Panel.Width(panelWidth).Render(strings.Join(content, "\n"))
}

func (m Model) nextFocus(key string) focusField {
	if key == "shift+tab" || key == "up" {
		if m.focus == focusEmail {
			return focusPassword
		}
		return focusEmail
	}

	if m.focus == focusPassword {
		return focusEmail
	}

	return focusPassword
}

func (m *Model) insertText(text string) {
	if m.focus == focusPassword {
		m.password += text
		return
	}

	m.email += text
}

func (m *Model) deleteRune() {
	if m.focus == focusPassword {
		m.password = trimLastRune(m.password)
		return
	}

	m.email = trimLastRune(m.email)
}

func fieldView(th theme.Theme, label string, value string, focused bool, secret bool) string {
	if value == "" {
		if secret {
			value = "Enter password"
		} else {
			value = "Enter email"
		}
		value = th.Muted.Render(value)
	}

	prefix := "  "
	if focused {
		prefix = th.Accent.Render("->")
	} else {
		prefix = th.Muted.Render("  ")
	}

	return strings.Join([]string{
		th.Muted.Render(label),
		prefix + " " + th.Body.Render(value),
	}, "\n")
}

func masked(value string) string {
	if value == "" {
		return ""
	}

	return strings.Repeat("*", utf8.RuneCountInString(value))
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}

	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 {
		return ""
	}

	return value[:len(value)-size]
}

func clampWidth(width int, fallback int) int {
	if width <= 0 {
		return fallback
	}
	if width < 40 {
		return width
	}
	if width < fallback {
		return width - 4
	}
	return fallback
}
