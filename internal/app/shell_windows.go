//go:build windows

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/papermap/papermap-tui/internal/config"
)

// errPwshNotInstalled is returned by resolveUserShell (and the
// preflight validator) when shell.windows is "pwsh" but no pwsh.exe
// can be found under any %ProgramFiles%\PowerShell\<N> directory.
// Treated as a fail-loud condition at TUI startup so the user fixes
// config or installs PowerShell 7 before reaching the chat screen.
var errPwshNotInstalled = errors.New("shell.windows is \"pwsh\" but pwsh.exe was not found under %ProgramFiles%\\PowerShell\\<N>; install PowerShell 7+ or set shell.windows: cmd")

// resolveUserShell returns the absolute path of the shell to invoke
// for "!" commands on Windows. Selection is driven entirely by
// config.Config.Shell.Windows; we never honor %COMSPEC%, never call
// exec.LookPath, and never probe PATH (CVE-2022-30580 class).
//
// Resolution rules:
//
//   - "pwsh" (default): glob %ProgramFiles%\PowerShell\*\pwsh.exe,
//     pick the highest version directory whose binary exists. Returns
//     errPwshNotInstalled if none found — user must install PowerShell
//     7+ or switch shell.windows to "cmd".
//   - "cmd": pin to %SystemRoot%\System32\cmd.exe.
//
// Unknown values would have been rejected by config.Load. Both
// resolved targets live under directories writable only by
// Administrators / TrustedInstaller, so a non-admin attacker cannot
// substitute the binary.
func resolveUserShell(cfg config.Config) (string, error) {
	switch cfg.Shell.Windows {
	case config.ShellWindowsCmd:
		return systemCmdExe(), nil
	case config.ShellWindowsPwsh, "":
		path, ok := findPwsh()
		if !ok {
			return "", errPwshNotInstalled
		}
		return path, nil
	default:
		return "", fmt.Errorf("invalid shell.windows %q", cfg.Shell.Windows)
	}
}

// systemCmdExe returns the canonical cmd.exe path under
// %SystemRoot%\System32. Falls back to the literal C:\Windows path
// when SystemRoot is unset, which keeps the resolver deterministic
// even on misconfigured machines.
func systemCmdExe() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "cmd.exe")
}

// findPwsh searches %ProgramFiles%\PowerShell\<version>\pwsh.exe and
// returns the highest-version match that exists on disk. Versions are
// compared as left-aligned numeric components ("7" < "10") so a
// future PowerShell 8/9/10 is picked over PowerShell 7 automatically.
//
// The %ProgramFiles%\PowerShell\ tree is admin-write-only on stock
// Windows installs, so any pwsh.exe present in a versioned subdir
// came from a signed MSI install and is safe to invoke.
func findPwsh() (string, bool) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	entries, err := os.ReadDir(filepath.Join(pf, "PowerShell"))
	if err != nil {
		return "", false
	}
	type candidate struct {
		version []int
		path    string
	}
	var found []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		version, ok := parseVersionDir(entry.Name())
		if !ok {
			continue
		}
		path := filepath.Join(pf, "PowerShell", entry.Name(), "pwsh.exe")
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		found = append(found, candidate{version: version, path: path})
	}
	if len(found) == 0 {
		return "", false
	}
	sort.Slice(found, func(i, j int) bool {
		return compareVersion(found[i].version, found[j].version) > 0
	})
	return found[0].path, true
}

// parseVersionDir parses directory names like "7", "7.4", "10.0.1"
// into a slice of components. Returns false for non-numeric names so
// stray directories under %ProgramFiles%\PowerShell\ (preview
// installs, leftover artifacts) do not confuse the picker.
func parseVersionDir(name string) ([]int, bool) {
	if name == "" {
		return nil, false
	}
	parts := strings.Split(name, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, false
		}
		n, err := atoiStrict(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// atoiStrict rejects any input that is not a pure non-negative
// decimal. strconv.Atoi accepts leading "+" / "-" which we do not
// want for version components.
func atoiStrict(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit in %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// compareVersion returns >0 when a is newer than b, <0 when older,
// 0 when equal. Shorter slices are zero-padded so "7" == "7.0.0".
func compareVersion(a, b []int) int {
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	for i := 0; i < max; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}
