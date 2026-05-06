// Package clitheme exposes shared huh form styling used by the
// `papermap` CLI subcommands (auth, workspace). The palette is duplicated
// from internal/theme rather than imported to keep this package free of
// the bubbletea-only theme struct.
package clitheme

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// PapermapHuh builds a huh theme tinted with the Papermap palette so the
// CLI forms do not look like a stock huh demo.
func PapermapHuh() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		s := huh.ThemeBase(isDark)

		accent := lipgloss.Color("#2ED8A3")
		soft := lipgloss.Color("#7BE7C5")
		muted := lipgloss.Color("#97A6A8")
		text := lipgloss.Color("#F2F5F4")
		errColor := lipgloss.Color("#FF7A7A")

		s.Focused.Title = s.Focused.Title.Foreground(accent).Bold(true)
		s.Focused.NoteTitle = s.Focused.NoteTitle.Foreground(accent)
		s.Focused.Description = s.Focused.Description.Foreground(muted)
		s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(soft)
		s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(accent)
		s.Focused.TextInput.Placeholder = s.Focused.TextInput.Placeholder.Foreground(muted)
		s.Focused.TextInput.Text = s.Focused.TextInput.Text.Foreground(text)
		s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(accent)
		s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(accent)
		s.Focused.ErrorIndicator = s.Focused.ErrorIndicator.Foreground(errColor)
		s.Focused.ErrorMessage = s.Focused.ErrorMessage.Foreground(errColor)
		s.Focused.Base = s.Focused.Base.BorderForeground(accent)

		s.Blurred.Title = s.Blurred.Title.Foreground(muted)
		s.Blurred.Description = s.Blurred.Description.Foreground(muted)
		s.Blurred.TextInput.Text = s.Blurred.TextInput.Text.Foreground(muted)

		s.Help.Ellipsis = s.Help.Ellipsis.Foreground(muted)
		s.Help.ShortKey = s.Help.ShortKey.Foreground(soft)
		s.Help.ShortDesc = s.Help.ShortDesc.Foreground(muted)
		s.Help.ShortSeparator = s.Help.ShortSeparator.Foreground(muted)

		return s
	})
}
