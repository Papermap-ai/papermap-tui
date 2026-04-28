package toast_test

import (
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/components/toast"
)

// TestShowMakesToastVisible verifies the basic Show -> Visible
// transition and that View renders non-empty content while live.
func TestShowMakesToastVisible(t *testing.T) {
	t.Parallel()

	m := toast.New()
	if m.Visible() {
		t.Fatal("zero-value toast should not be visible")
	}

	cmd := m.Show("Copied", toast.KindSuccess, 0)
	if cmd == nil {
		t.Fatal("Show should return a dismiss tick cmd")
	}
	if !m.Visible() {
		t.Fatal("toast should be visible after Show")
	}
	if v := m.View(theme.Default()); v == "" {
		t.Fatal("View should render non-empty text while visible")
	}
}

// TestDismissTickHidesToast verifies the dismiss message returned by
// the Show tick clears the toast.
func TestDismissTickHidesToast(t *testing.T) {
	t.Parallel()

	m := toast.New()
	cmd := m.Show("Copied", toast.KindSuccess, time.Millisecond)
	dismissed := m.Update(cmd())
	if !dismissed {
		t.Fatal("Update should report dismissed=true for matching tick")
	}
	if m.Visible() {
		t.Fatal("toast should be hidden after dismiss tick")
	}
	if v := m.View(theme.Default()); v != "" {
		t.Fatalf("View should be empty after dismiss, got %q", v)
	}
}

// TestStaleDismissIgnored verifies that a dismiss tick from an older
// Show is ignored once a newer Show has bumped the token. Without
// the token guard, the second toast would vanish immediately when
// the first toast's pending tick finally fires.
func TestStaleDismissIgnored(t *testing.T) {
	t.Parallel()

	m := toast.New()
	stale := m.Show("first", toast.KindSuccess, 0)
	_ = m.Show("second", toast.KindSuccess, 0)

	dismissed := m.Update(stale())
	if dismissed {
		t.Fatal("stale dismiss should be ignored")
	}
	if !m.Visible() {
		t.Fatal("newer toast should remain visible")
	}
}
