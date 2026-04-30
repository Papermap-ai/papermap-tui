//go:build windows

package shell

import (
	"os/exec"
	"sync"

	"golang.org/x/sys/windows"
)

// jobRegistry maps an *exec.Cmd to its Job Object handle for the
// duration of a single Run invocation. Entries are added by
// configureProcess and removed by the cleanup it returns. Using a
// per-process registry avoids stashing platform handles in
// exec.Cmd.SysProcAttr (where they would survive across Cmd reuse)
// and keeps the cancel/cleanup closures small.
var (
	jobRegistryMu sync.Mutex
	jobRegistry   = map[*exec.Cmd]windows.Handle{}
)

func registerJob(c *exec.Cmd, h windows.Handle) {
	jobRegistryMu.Lock()
	defer jobRegistryMu.Unlock()
	jobRegistry[c] = h
}

func unregisterJob(c *exec.Cmd) {
	jobRegistryMu.Lock()
	defer jobRegistryMu.Unlock()
	delete(jobRegistry, c)
}

func lookupJob(c *exec.Cmd) windows.Handle {
	jobRegistryMu.Lock()
	defer jobRegistryMu.Unlock()
	return jobRegistry[c]
}
