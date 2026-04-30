//go:build windows

package shell

import (
	"reflect"
	"testing"
)

func TestShellArgsPwsh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
	}{
		{"basename pwsh.exe", `C:\Program Files\PowerShell\7\pwsh.exe`},
		{"basename pwsh", `C:\Program Files\PowerShell\7\pwsh`},
		{"case insensitive", `C:\Program Files\PowerShell\7\PWSH.EXE`},
	}
	want := []string{"-NoProfile", "-NonInteractive", "-NoLogo", "-Command", "echo hello"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shellArgs(tc.path, "echo hello")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("shellArgs(%q) = %v, want %v", tc.path, got, want)
			}
		})
	}
}

func TestShellArgsCmd(t *testing.T) {
	t.Parallel()
	got := shellArgs(`C:\Windows\System32\cmd.exe`, "echo hello")
	want := []string{"/C", "echo hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shellArgs cmd.exe = %v, want %v", got, want)
	}
}

func TestShellArgsUnknownFallsToCmdRecipe(t *testing.T) {
	t.Parallel()
	// A renamed binary or unexpected path should fall through to
	// the cmd.exe recipe rather than guess. The /C will fail
	// loudly when invoked against PowerShell, which is the
	// intended behavior.
	got := shellArgs(`C:\Windows\System32\powershell.exe`, "echo hello")
	want := []string{"/C", "echo hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shellArgs unknown = %v, want %v", got, want)
	}
}
