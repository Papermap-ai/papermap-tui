package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/shell"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

// shellResultMsg carries the outcome of a "!" shell-mode command from
// the worker goroutine back into the Bubble Tea Update loop. The
// payload is the same struct the chat layer renders, plus the error
// from shell.Run so the handler can compose a stable footer.
type shellResultMsg struct {
	result shell.Result
	err    error
}

// startShellCommand validates $SHELL, kicks off shell.Run on a worker
// goroutine, and stores the cancel func so esc can tear it down. The
// returned Cmd resolves to a shellResultMsg once the command exits.
//
// The chat layer has already latched shellRunning before this runs, so
// we do not touch chat state here. All transcript mutation happens in
// handleShellResult once the command lands.
func (m *Model) startShellCommand(cmdLine string) tea.Cmd {
	shellPath := resolveUserShell()
	ctx, cancel := context.WithCancel(context.Background())
	m.shellCancel = cancel
	return func() tea.Msg {
		res, err := shell.Run(ctx, shellPath, cmdLine, shell.DefaultCap)
		return shellResultMsg{result: res, err: err}
	}
}

// cancelShellCommand fires the worker's cancel func. Bubble Tea will
// still receive a shellResultMsg once shell.Run unwinds, and the
// handler relies on that delivery to flip shellRunning off and append
// the (cancelled) transcript turn.
func (m *Model) cancelShellCommand() {
	if m.shellCancel != nil {
		m.shellCancel()
		m.shellCancel = nil
	}
}

func (m Model) handleShellResult(msg shellResultMsg) Model {
	m.shellCancel = nil
	res := msg.result
	errorText := ""
	if msg.err != nil {
		switch {
		case errors.Is(msg.err, context.Canceled),
			errors.Is(msg.err, context.DeadlineExceeded):
			errorText = "cancelled"
		case res.Truncated:
			// Truncation already surfaced inline; the muted
			// footer can stay quiet so we do not double-shout.
		default:
			errorText = msg.err.Error()
		}
	}
	m.chat.AppendShellResult(chat.ShellResult{
		Command:   res.Command,
		Output:    res.Output,
		ExitCode:  res.ExitCode,
		Duration:  res.Duration,
		Truncated: res.Truncated,
		CapBytes:  res.CapBytes,
		ErrorText: errorText,
	})
	return m
}

// resolveUserShell picks the shell binary to invoke for "!" commands.
// $SHELL wins when it points at an existing executable file; otherwise
// /bin/sh is the fallback so the feature still works on minimal
// environments. Validating the path here keeps shell.Run focused on
// execution semantics.
func resolveUserShell() string {
	candidate := strings.TrimSpace(os.Getenv("SHELL"))
	if candidate == "" {
		return "/bin/sh"
	}
	if !filepath.IsAbs(candidate) {
		return "/bin/sh"
	}
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "/bin/sh"
	}
	if info.Mode()&0o111 == 0 {
		return "/bin/sh"
	}
	return candidate
}
