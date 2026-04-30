package shell

import (
	"strings"
	"testing"
)

func TestSanitizeStripsOSC52Clipboard(t *testing.T) {
	t.Parallel()
	in := "before\x1b]52;c;c2VjcmV0\x07after"
	got := sanitizeANSI(in)
	if strings.Contains(got, "52") || strings.Contains(got, "c2VjcmV0") {
		t.Fatalf("OSC 52 leaked: %q", got)
	}
	if got != "beforeafter" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeStripsOSC8Hyperlink(t *testing.T) {
	t.Parallel()
	in := "\x1b]8;;https://evil/\x07click\x1b]8;;\x07 done"
	got := sanitizeANSI(in)
	if strings.Contains(got, "evil") || strings.Contains(got, "https") {
		t.Fatalf("hyperlink leaked: %q", got)
	}
	if got != "click done" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeStripsCSIAndSGR(t *testing.T) {
	t.Parallel()
	in := "\x1b[31mred\x1b[0m text\x1b[2J"
	got := sanitizeANSI(in)
	if got != "red text" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeStripsDCSAndAPC(t *testing.T) {
	t.Parallel()
	in := "a\x1bP1;2qpayload\x1b\\b\x1b_apc payload\x1b\\c"
	got := sanitizeANSI(in)
	if got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizePreservesWhitespace(t *testing.T) {
	t.Parallel()
	in := "line1\nline2\tcol\r\n"
	got := sanitizeANSI(in)
	if got != in {
		t.Fatalf("whitespace mangled: %q", got)
	}
}

func TestSanitizeStripsBareControls(t *testing.T) {
	t.Parallel()
	in := "ok\x07bell\x00nul"
	got := sanitizeANSI(in)
	if got != "okbellnul" {
		t.Fatalf("got %q", got)
	}
}
