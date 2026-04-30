//go:build windows

package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/papermap/papermap-tui/internal/config"
)

// withProgramFiles points %ProgramFiles% at a temp tree so findPwsh
// reads from the test fixture instead of the real install. Required
// because the resolver intentionally does not accept a custom root —
// keeping the search path admin-write-only is part of the security
// posture.
func withProgramFiles(t *testing.T) string {
	t.Helper()
	pf := t.TempDir()
	t.Setenv("ProgramFiles", pf)
	return pf
}

func makePwshDir(t *testing.T, pf, version string) string {
	t.Helper()
	dir := filepath.Join(pf, "PowerShell", version)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "pwsh.exe")
	if err := os.WriteFile(path, []byte("stub"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestResolveUserShellCmdReturnsSystemCmdExe(t *testing.T) {
	t.Setenv("SystemRoot", `C:\TestRoot`)
	got, err := resolveUserShell(config.Config{Shell: config.ShellConfig{Windows: config.ShellWindowsCmd}})
	if err != nil {
		t.Fatalf("resolveUserShell: %v", err)
	}
	want := filepath.Join(`C:\TestRoot`, "System32", "cmd.exe")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveUserShellPwshFindsHighestVersion(t *testing.T) {
	pf := withProgramFiles(t)
	makePwshDir(t, pf, "7")
	want := makePwshDir(t, pf, "10.0.1")
	makePwshDir(t, pf, "7.4")

	got, err := resolveUserShell(config.Config{Shell: config.ShellConfig{Windows: config.ShellWindowsPwsh}})
	if err != nil {
		t.Fatalf("resolveUserShell: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q (highest version)", got, want)
	}
}

func TestResolveUserShellPwshDefaultsWhenEmpty(t *testing.T) {
	pf := withProgramFiles(t)
	want := makePwshDir(t, pf, "7")

	got, err := resolveUserShell(config.Config{})
	if err != nil {
		t.Fatalf("resolveUserShell: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveUserShellPwshFailsLoudWhenMissing(t *testing.T) {
	withProgramFiles(t)
	_, err := resolveUserShell(config.Config{Shell: config.ShellConfig{Windows: config.ShellWindowsPwsh}})
	if !errors.Is(err, errPwshNotInstalled) {
		t.Fatalf("got err %v, want errPwshNotInstalled", err)
	}
}

func TestResolveUserShellRejectsInvalidValue(t *testing.T) {
	_, err := resolveUserShell(config.Config{Shell: config.ShellConfig{Windows: "powershell"}})
	if err == nil {
		t.Fatal("expected error for invalid shell.windows")
	}
}

func TestFindPwshSkipsNonVersionDirs(t *testing.T) {
	pf := withProgramFiles(t)
	// Stray dir that should be ignored.
	if err := os.MkdirAll(filepath.Join(pf, "PowerShell", "preview"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := makePwshDir(t, pf, "7")

	got, ok := findPwsh()
	if !ok {
		t.Fatal("findPwsh returned false")
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindPwshSkipsVersionDirsWithoutBinary(t *testing.T) {
	pf := withProgramFiles(t)
	// Create a "10" dir with no pwsh.exe — should be skipped so 7
	// wins.
	if err := os.MkdirAll(filepath.Join(pf, "PowerShell", "10"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := makePwshDir(t, pf, "7")

	got, ok := findPwsh()
	if !ok {
		t.Fatal("findPwsh returned false")
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseVersionDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		want   []int
		wantOK bool
	}{
		{"single digit", "7", []int{7}, true},
		{"two parts", "7.4", []int{7, 4}, true},
		{"three parts", "10.0.1", []int{10, 0, 1}, true},
		{"empty", "", nil, false},
		{"non-numeric", "preview", nil, false},
		{"trailing dot", "7.", nil, false},
		{"leading sign", "+7", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseVersionDir(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompareVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b []int
		want int // sign only
	}{
		{"7 vs 10", []int{7}, []int{10}, -1},
		{"10 vs 7", []int{10}, []int{7}, 1},
		{"7 vs 7.0.0", []int{7}, []int{7, 0, 0}, 0},
		{"7.4 vs 7.3", []int{7, 4}, []int{7, 3}, 1},
		{"equal", []int{7, 4}, []int{7, 4}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareVersion(tc.a, tc.b)
			switch {
			case tc.want == 0 && got != 0:
				t.Fatalf("got %d, want 0", got)
			case tc.want > 0 && got <= 0:
				t.Fatalf("got %d, want >0", got)
			case tc.want < 0 && got >= 0:
				t.Fatalf("got %d, want <0", got)
			}
		})
	}
}
