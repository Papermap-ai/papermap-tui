package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/shell"
	"github.com/papermap/papermap-tui/internal/ui/chat"
)

// shellResultMsg carries the outcome of a "!" shell-mode command from
// the worker goroutine back into the Bubble Tea Update loop.
type shellResultMsg struct {
	result shell.Result
	err    error
}

// shellMaxDuration is the hard ceiling for any single "!" invocation.
// A user can still cancel earlier with esc; this exists to bound the
// blast radius of forgotten background loops or hung child processes
// that survived the cancel signal.
const shellMaxDuration = 10 * time.Minute

// startShellCommand uses the shell binary resolved at TUI startup
// (m.shellPath), scrubs PAPERMAP_* tokens from the inherited env,
// and kicks off shell.Run on a worker goroutine with a 10-minute
// timeout. The cancel func is stored on the model so esc can tear
// it down.
//
// All transcript mutation happens in handleShellResult once the
// command lands.
func (m *Model) startShellCommand(cmdLine string) tea.Cmd {
	shellPath := m.shellPath
	env := scrubChildEnv(os.Environ())
	ctx, cancel := context.WithTimeout(context.Background(), shellMaxDuration)
	m.shellCancel = cancel
	return func() tea.Msg {
		res, err := shell.Run(ctx, shellPath, cmdLine, shell.DefaultCap, env)
		return shellResultMsg{result: res, err: err}
	}
}

// cancelShellCommand fires the worker's cancel func. Bubble Tea will
// still receive a shellResultMsg once shell.Run unwinds.
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
			// Truncation already surfaced inline.
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

// scrubChildEnv removes PAPERMAP_* variables from the environment we
// hand to the spawned shell. Papermap-shaped vars should not leak into
// a user-typed `! env` or any process the command spawns.
func scrubChildEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "PAPERMAP_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
