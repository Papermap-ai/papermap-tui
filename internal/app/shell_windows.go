//go:build windows

package app

import (
	"os"
	"path/filepath"
)

// resolveUserShell picks the shell binary for "!" commands on
// Windows. We deliberately do NOT honor %COMSPEC% — it is
// env-controlled and trivially poisoned by anything that wrote to the
// user's environment block. Instead we resolve cmd.exe under
// %SystemRoot%\System32, which is a directory only Administrators
// (and TrustedInstaller) can write to.
//
// We also do NOT call exec.LookPath: CVE-2022-30580 demonstrated that
// resolving "cmd" via PATH can pick up a user-writable cmd.exe in the
// current working directory on older Go versions. Pinning the
// absolute path eliminates the lookup entirely.
//
// PowerShell is intentionally not yet wired up here. A future config
// key (Shell.Windows = "powershell") plus an install-path allowlist
// will dispatch into pwsh.exe / powershell.exe under their canonical
// install directories with -NoProfile -NonInteractive -NoLogo
// -Command. Until that lands every "!" invocation is cmd.exe.
func resolveUserShell() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "cmd.exe")
}
