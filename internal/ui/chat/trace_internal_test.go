package chat

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/theme"
)

func TestTruncateHead(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"shorter than n", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello"},
		{"zero", "hello", 0, ""},
		{"negative", "hello", -1, ""},
		{"multibyte", "héllo wörld", 5, "héllo"},
		{"emoji rune", "ab😀cd", 3, "ab😀"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateHead(tc.in, tc.n)
			if got != tc.want {
				t.Fatalf("truncateHead(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}

func TestLatestThinkingSnippet(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := latestThinkingSnippet(nil); got != "" {
			t.Fatalf("expected empty snippet, got %q", got)
		}
	})

	t.Run("returns latest non-empty thought", func(t *testing.T) {
		t.Parallel()
		steps := []TraceStep{
			{Kind: TraceThought, Body: "first thought"},
			{Kind: TraceToolCall, Title: "Run SQL"},
			{Kind: TraceThought, Body: "  second   thought  "},
		}
		got := latestThinkingSnippet(steps)
		if got != "second thought" {
			t.Fatalf("expected whitespace-collapsed second thought, got %q", got)
		}
	})

	t.Run("falls back to latest tool title when no thoughts", func(t *testing.T) {
		t.Parallel()
		steps := []TraceStep{
			{Kind: TraceToolCall, Title: "First Tool"},
			{Kind: TraceToolCall, Title: "Latest Tool"},
		}
		got := latestThinkingSnippet(steps)
		if got != "Latest Tool" {
			t.Fatalf("expected latest tool title, got %q", got)
		}
	})

	t.Run("ignores empty thought body", func(t *testing.T) {
		t.Parallel()
		steps := []TraceStep{
			{Kind: TraceThought, Body: "earlier"},
			{Kind: TraceThought, Body: "   "},
		}
		got := latestThinkingSnippet(steps)
		if got != "earlier" {
			t.Fatalf("expected earlier thought when latest is blank, got %q", got)
		}
	})
}

func TestMutedThinkingPreviewTruncates(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a", 200)
	out := mutedThinkingPreview(theme.Default(), []TraceStep{
		{Kind: TraceThought, Body: body},
	})
	// Preview always carries the prefix and trailing ellipsis. The
	// truncation cap is 60 runes of the snippet itself.
	if !strings.Contains(out, "· thinking ") {
		t.Fatalf("expected preview prefix, got %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis somewhere in preview, got %q", out)
	}
	// Count 'a' runes that ended up inside the rendered string. With a
	// 60-rune cap we should see at most 60.
	count := strings.Count(out, "a")
	if count > 60 {
		t.Fatalf("expected at most 60 a runes after truncation, got %d", count)
	}
}

func TestMutedThinkingPreviewEmpty(t *testing.T) {
	t.Parallel()

	out := mutedThinkingPreview(theme.Default(), nil)
	if !strings.Contains(out, "· thinking…") {
		t.Fatalf("expected static placeholder, got %q", out)
	}
}
