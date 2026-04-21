package app

import (
	tea "charm.land/bubbletea/v2"
)

// StartupMsgFromInit runs Init() on the provided model and returns the
// startupMsg produced by the loadStartup command. Test-only; lives in
// export_test.go so it never ships in the binary.
func StartupMsgFromInit(m Model) (tea.Msg, bool) {
	cmd := m.Init()
	if cmd == nil {
		return nil, false
	}
	return findStartupMsg(cmd)
}

func findStartupMsg(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	msg := cmd()
	switch v := msg.(type) {
	case startupMsg:
		return v, true
	case tea.BatchMsg:
		for _, sub := range v {
			if got, ok := findStartupMsg(sub); ok {
				return got, true
			}
		}
	}
	return nil, false
}
