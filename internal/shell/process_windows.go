//go:build windows

package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SECURITY: Errors from this file MUST NOT include the user's command
// string or any portion of process stdout/stderr. See package doc.
//
// Job Object invariants — DO NOT change without re-running the
// security review:
//
//   - JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE is set so closing the handle
//     synchronously kills the entire process tree.
//   - JOB_OBJECT_LIMIT_BREAKAWAY_OK and JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK
//     are intentionally NOT set: a grandchild attempting CREATE_BREAKAWAY_FROM_JOB
//     must fail so it cannot escape our cleanup.
//   - SECURITY_ATTRIBUTES is nil (default), so the job handle is not
//     inheritable.
//   - We do NOT set CREATE_BREAKAWAY_FROM_JOB on the child.
//   - We do NOT use CREATE_SUSPENDED. The microsecond-scale window
//     between Start and AssignProcessToJobObject is accepted because
//     (a) the user is the attacker model for "!" mode, (b) there are
//     simpler escape vectors available to a user typing arbitrary
//     shell commands, and (c) closing the job handle on cleanup
//     catches anything we missed.

// defaultShellPath returns the hardened cmd.exe path under
// %SystemRoot%\System32. We deliberately ignore %COMSPEC%
// (env-controlled, trivially poisoned) and never call exec.LookPath
// (CVE-2022-30580 class). If the resolved file does not exist we
// still return the literal path; exec.Command will surface a clean
// "file not found" error rather than executing something unexpected.
func defaultShellPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "cmd.exe")
}

// shellArgs builds the argv for shellPath. PowerShell support is
// behind a future config key (Shell.Windows = "powershell") + an
// install-path allowlist; until that ships, every invocation is
// cmd.exe and renamed binaries would fall through to /C and fail
// cleanly rather than execute anything unexpected.
//
// TODO(papermap-tui#shell-windows-pwsh): when the config key lands,
// dispatch on the resolved binary name only after verifying its
// absolute path is under
// %SystemRoot%\System32\WindowsPowerShell\v1.0\ or
// %ProgramFiles%\PowerShell\, and use:
//
//	[]string{"-NoProfile", "-NonInteractive", "-NoLogo", "-Command", cmd}
//
// -NoProfile is REQUIRED — without it every "!" invocation runs the
// user's $PROFILE script, which defeats the point of a one-shot.
func shellArgs(_ string, cmd string) []string {
	return []string{"/C", cmd}
}

// configureProcess creates a Job Object, wires Cmd.Cancel to
// terminate it, and returns a cleanup that closes the handle (which,
// with KILL_ON_JOB_CLOSE, atomically kills any surviving children).
// The returned hook is captured by runUnderJob via the *jobHandle
// returned through Cmd-scoped state in the closure.
func configureProcess(c *exec.Cmd, _ context.CancelFunc) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() {}, fmt.Errorf("create job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return func() {}, fmt.Errorf("set job limits: %w", err)
	}

	// CREATE_NEW_PROCESS_GROUP isolates the child from console
	// signals (Ctrl+C/Ctrl+Break to the TUI) and would let us
	// deliver CTRL_BREAK_EVENT for soft cancel later. Today we
	// hard-kill via the job, but the process-group flag is cheap
	// future-proofing.
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}

	// Stash the handle on the cmd via a per-Cmd registration. The
	// runUnderJob hook reads it back to assign the started process
	// to the job before Wait.
	registerJob(c, job)

	c.Cancel = func() error {
		// Preferred path: tell the kernel to kill the whole job.
		if err := windows.TerminateJobObject(job, 1); err == nil {
			return nil
		}
		// Last-resort: kill just the root child. The cleanup
		// closure will CloseHandle the job, which (via
		// KILL_ON_JOB_CLOSE) catches anything we missed.
		if c.Process != nil {
			return c.Process.Kill()
		}
		return nil
	}
	c.WaitDelay = 250 * time.Millisecond

	cleanup := func() {
		unregisterJob(c)
		_ = windows.CloseHandle(job)
	}
	return cleanup, nil
}

// runUnderJob runs the Windows orchestration: Start the process, then
// immediately assign it to the job. There is a microsecond-scale race
// here where a fast child could spawn a grandchild before assignment;
// see the file-level comment for why we accept it. If assignment
// fails we fail-closed by killing the child and returning an error
// without including the command text.
func runUnderJob(c *exec.Cmd) error {
	if err := c.Start(); err != nil {
		return err
	}
	job := lookupJob(c)
	if job != 0 && c.Process != nil {
		ph, err := windows.OpenProcess(
			windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
			false,
			uint32(c.Process.Pid),
		)
		if err == nil {
			assignErr := windows.AssignProcessToJobObject(job, ph)
			_ = windows.CloseHandle(ph)
			if assignErr != nil {
				_ = c.Process.Kill()
				_ = c.Wait()
				return fmt.Errorf("assign job: %w", assignErr)
			}
		}
		// If OpenProcess fails the child may have already exited
		// (very fast `cmd /C echo`) — fall through to Wait, which
		// collects the exit status normally.
	}
	return c.Wait()
}
