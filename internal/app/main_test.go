package app_test

import (
	"os"
	"testing"
)

// TestMain forces the file-backed credential store for the entire app
// test binary. The keyring backend would otherwise prompt the macOS
// Keychain (or fail in headless CI), neither of which we want during
// tests. Force-file is read by auth.DefaultStore at construction time.
func TestMain(m *testing.M) {
	_ = os.Setenv("PAPERMAP_FORCE_FILE_STORE", "1")
	os.Exit(m.Run())
}
