//go:build unix

package shell

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// SECURITY: Errors from this file MUST NOT include the user's command
// string or any portion of process stdout/stderr. See package doc.

// defaultShellPath is the Unix fallback when the caller passes an
// empty shellPath. Mirrors the long-standing convention that /bin/sh
// is always present on a POSIX system.
func defaultShellPath() string { return "/bin/sh" }

// shellArgs returns the argv to invoke shellPath as a one-shot
// command runner. Unix shells universally support `-c`.
func shellArgs(_ string, cmd string) []string {
	return []string{"-c", cmd}
}

// configureProcess wires the Unix process-group + cancel behavior
// onto c. The child runs in a fresh process group via Setpgid so
// negative-pid kill targets the whole tree, including any backgrounded
// children the shell spawns (e.g. `sleep 60 &`).
//
// Returns a no-op cleanup; Unix does not need post-Wait teardown.
func configureProcess(c *exec.Cmd, _ context.CancelFunc) (func(), error) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		// Kill the entire process group. The negative pid form
		// targets the group leader's group, catching any
		// backgrounded children spawned by the shell. ESRCH
		// means the process already exited, which is benign.
		if err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL); err == nil {
			return nil
		} else if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		// Fall back to direct kill in case Setpgid was not honored.
		if err := c.Process.Kill(); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	// Give the cancel signal a brief window to land before Wait
	// returns. Without WaitDelay the goroutine reading stdout can
	// hang on a child that ignores SIGTERM.
	c.WaitDelay = 250 * time.Millisecond
	return func() {}, nil
}

// runUnderJob is the Unix orchestration: a plain Run is sufficient
// because the process group is configured before exec.
func runUnderJob(c *exec.Cmd) error {
	return c.Run()
}
