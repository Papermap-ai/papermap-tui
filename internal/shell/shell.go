// Package shell runs one-shot user shell commands from the chat input
// "!" mode. It is intentionally narrow: a single Run entry point that
// accepts a context, the shell to invoke, the command string, and an
// output cap. The result struct carries everything the chat layer needs
// to render a transient turn.
//
// The package never logs the command text or its output. The AGENTS.md
// credential rules apply even though shell payloads are not credentials:
// users may paste secrets into a "!" line, and we do not want them in
// crash dumps or panic traces.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Result is the outcome of a single shell invocation. Output has already
// been ANSI-sanitized and capped. Truncated is true when the process
// produced more bytes than the cap allowed; the writer kills the
// process in that case so a runaway command (e.g. `yes`) cannot
// exhaust memory.
type Result struct {
	Command   string
	Output    string
	ExitCode  int
	Duration  time.Duration
	Truncated bool
	// CapBytes is the byte budget that was applied. Surfaced so the
	// renderer can compose a stable truncation marker without the
	// caller having to remember the value passed to Run.
	CapBytes int
}

// DefaultCap is the per-command output budget. Sized to comfortably
// hold a few screens of text without letting an unbounded stream pin
// the TUI process.
const DefaultCap = 256 * 1024

// errOutputCapExceeded is returned by cappedWriter once it has accepted
// CapBytes worth of output. The caller treats it as a signal to kill
// the child; it never reaches the user.
var errOutputCapExceeded = errors.New("shell: output cap exceeded")

// Run executes cmd via `shellPath -c cmd` with a fresh process group so
// the whole tree can be killed on cancel or cap overflow. The returned
// Result is always populated even on error so the chat layer can render
// a stable failure block.
//
// shellPath should be a validated absolute path to an executable. When
// it is empty Run falls back to /bin/sh.
func Run(ctx context.Context, shellPath, cmd string, capBytes int) (Result, error) {
	if capBytes <= 0 {
		capBytes = DefaultCap
	}
	if strings.TrimSpace(shellPath) == "" {
		shellPath = "/bin/sh"
	}

	start := time.Now()
	res := Result{Command: cmd, CapBytes: capBytes}

	// A child context lets the capped writer cancel the process via
	// Cmd.Cancel without disturbing the caller's ctx semantics.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	c := exec.CommandContext(runCtx, shellPath, "-c", cmd)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		// Kill the entire process group. The negative pid form of
		// syscall.Kill targets the process group leader's group,
		// catching any backgrounded children spawned by the shell
		// (e.g. `sleep 60 &`). Falling back to Process.Kill keeps
		// us correct on the off chance Setpgid was not honored.
		if c.Process != nil {
			if err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL); err == nil {
				return nil
			}
			return c.Process.Kill()
		}
		return nil
	}
	// Give the cancel signal a brief window to land before Wait
	// returns. Without WaitDelay the goroutine reading stdout can
	// hang on a child that ignores SIGTERM.
	c.WaitDelay = 250 * time.Millisecond

	buf := &bytes.Buffer{}
	w := newCappedWriter(buf, capBytes, cancel)
	c.Stdout = w
	c.Stderr = w

	err := c.Run()
	res.Duration = time.Since(start)
	res.ExitCode = c.ProcessState.ExitCode()
	res.Truncated = w.truncated()
	res.Output = sanitizeANSI(buf.String())

	switch {
	case err == nil:
		return res, nil
	case res.Truncated:
		// Cap-driven kill is the expected path; surface a clean
		// error so callers can distinguish from a real failure.
		return res, fmt.Errorf("output exceeded %d bytes", capBytes)
	case errors.Is(ctx.Err(), context.Canceled):
		return res, context.Canceled
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return res, context.DeadlineExceeded
	default:
		return res, err
	}
}

// cappedWriter forwards bytes to an underlying buffer until limit is
// reached, then triggers cancel and refuses further writes. The writer
// is safe for concurrent use by stdout and stderr goroutines.
type cappedWriter struct {
	mu        sync.Mutex
	dst       io.Writer
	limit     int
	written   int
	cancel    context.CancelFunc
	overflow  bool
	cancelled bool
}

func newCappedWriter(dst io.Writer, limit int, cancel context.CancelFunc) *cappedWriter {
	return &cappedWriter{dst: dst, limit: limit, cancel: cancel}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.overflow {
		return 0, errOutputCapExceeded
	}
	remaining := w.limit - w.written
	if remaining <= 0 {
		w.overflow = true
		w.fire()
		return 0, errOutputCapExceeded
	}
	if len(p) <= remaining {
		n, err := w.dst.Write(p)
		w.written += n
		return n, err
	}
	n, _ := w.dst.Write(p[:remaining])
	w.written += n
	w.overflow = true
	w.fire()
	return n, errOutputCapExceeded
}

func (w *cappedWriter) fire() {
	if w.cancelled {
		return
	}
	w.cancelled = true
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *cappedWriter) truncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.overflow
}
