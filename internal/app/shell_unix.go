//go:build unix

package app

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveUserShell picks the shell binary to invoke for "!" commands.
// $SHELL wins when it points at an existing executable file; otherwise
// /bin/sh is the fallback so the feature still works on minimal
// environments.
//
// $SHELL is user-trusted on Unix: the same value drives every
// interactive shell session this user starts, so honoring it here is
// consistent with the rest of the system.
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
