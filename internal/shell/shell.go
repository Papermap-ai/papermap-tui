// Package shell runs one-shot user shell commands from the chat input
// "!" mode. It is intentionally narrow: a single Run entry point that
// accepts a context, the shell to invoke, the command string, and an
// output cap. The result struct carries everything the chat layer needs
// to render a transient turn.
//
// Per-OS process control lives in process_unix.go and process_windows.go;
// per-OS escape-introducer sets live in escapes_unix.go and escapes_windows.go.
//
// SECURITY: Errors returned from this package MUST NOT include the
// user's command string or any portion of process stdout/stderr. Users
// paste secrets into "!" mode, and we do not want them in crash dumps,
// panic traces, or wrapped errors. Wrap with operation name only
// (e.g. fmt.Errorf("assign job: %w", err)) — never include cmd, output,
// or any caller-supplied byte slice.
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
	"time"
)

// Result is the outcome of a single shell invocation. Output has already
// been ANSI-sanitized and capped. Truncated is true when the process
// produced more bytes than the cap allowed; the writer kills the
// process in that case so a runaway command (e.g. `yes`) cannot
// exhaust memory.
type Result struct {
	// SECURITY: Command is the raw user-typed shell command. It MUST
	// NOT be persisted to disk, sent to the backend, included in
	// crash reports, or otherwise transmitted. Future "export
	// transcript" features must redact "!" turns or omit them.
	Command string
	// SECURITY: Output is raw process stdout+stderr after ANSI
	// sanitization. Same constraints as Command — users paste
	// secrets into "!" lines and the output may echo them back
	// (e.g. `! echo $TOKEN`). Treat as untrusted-and-sensitive.
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

// errCommandHasNUL is returned when the caller passes a command
// containing a NUL byte. Windows CreateProcessW silently truncates at
// NUL, so we reject up front to keep the cross-OS behavior
// predictable. The error text is intentionally generic so it can be
// surfaced to the user without leaking the command.
var errCommandHasNUL = errors.New("shell: command contains NUL byte")

// Run executes cmd via shellPath using a per-OS argv recipe (e.g.
// `-c` on Unix, `/C` on cmd.exe, `-NoProfile -NonInteractive -NoLogo
// -Command` on PowerShell). On Unix the child runs in its own process
// group so the whole tree can be killed on cancel or cap overflow; on
// Windows the child is assigned to a Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so the kernel kills the tree
// when we close the handle. The returned Result is always populated
// even on error so the chat layer can render a stable failure block.
//
// shellPath should be a validated absolute path to an executable.
// When it is empty Run defers to the per-OS default (see
// defaultShellPath).
//
// env, when non-nil, replaces the inherited environment for the
// child. Pass nil to inherit os.Environ() unchanged. Callers that
// want to scrub PAPERMAP_* tokens from the child env should build the
// filtered slice themselves and pass it here.
//
// SECURITY: Do not include cmd or shellPath in any error wrapping.
// See package doc.
func Run(ctx context.Context, shellPath, cmd string, capBytes int, env []string) (Result, error) {
	if capBytes <= 0 {
		capBytes = DefaultCap
	}
	if strings.TrimSpace(shellPath) == "" {
		shellPath = defaultShellPath()
	}
	if strings.ContainsRune(cmd, 0x00) {
		return Result{Command: cmd, CapBytes: capBytes}, errCommandHasNUL
	}

	start := time.Now()
	res := Result{Command: cmd, CapBytes: capBytes}

	// A child context lets the capped writer cancel the process via
	// Cmd.Cancel without disturbing the caller's ctx semantics.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	args := shellArgs(shellPath, cmd)
	c := exec.CommandContext(runCtx, shellPath, args...)
	if env != nil {
		c.Env = env
	}
	// Stdin is intentionally nil so commands that read stdin (e.g.
	// `cat`, `more`) EOF immediately rather than hanging behind
	// WaitDelay. A user typing "! cat" should see an instant prompt
	// return, not a 250ms freeze on cancel.
	c.Stdin = nil

	buf := &bytes.Buffer{}
	w := newCappedWriter(buf, capBytes, cancel)
	c.Stdout = w
	c.Stderr = w

	cleanup, err := configureProcess(c, cancel)
	if err != nil {
		// configureProcess refused to set up (e.g. Windows Job
		// Object creation failed). Surface a clean error without
		// the command text.
		return res, fmt.Errorf("configure process: %w", err)
	}
	defer cleanup()

	err = runUnderJob(c)
	res.Duration = time.Since(start)
	if c.ProcessState != nil {
		res.ExitCode = c.ProcessState.ExitCode()
	} else {
		res.ExitCode = -1
	}
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

// fire triggers the cancel func exactly once. The cancel func is a
// context.CancelFunc which is panic-safe; if a future maintainer
// swaps in a closure, that closure must not panic — this writer holds
// the mutex while calling it.
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
