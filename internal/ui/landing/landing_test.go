package landing_test

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/landing"
)

func TestSignedOutLandingUsesQuitHint(t *testing.T) {
	t.Parallel()

	view := landing.NewModel().View(theme.Default(), 62, "Session expired")
	if !strings.Contains(view, "Ctrl+C quit") {
		t.Fatalf("expected Ctrl+C quit hint, got %q", view)
	}
	if strings.Contains(view, "Any key quit") {
		t.Fatalf("unexpected stale hint in view: %q", view)
	}
}
