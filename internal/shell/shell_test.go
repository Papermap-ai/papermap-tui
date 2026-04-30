package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunSuccess(t *testing.T) {
	t.Parallel()
	res, err := Run(context.Background(), "/bin/sh", "printf hello", 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Output != "hello" {
		t.Fatalf("output = %q want %q", res.Output, "hello")
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d want 0", res.ExitCode)
	}
	if res.Truncated {
		t.Fatalf("unexpected truncation")
	}
	if res.Duration <= 0 {
		t.Fatalf("duration not recorded")
	}
}

func TestRunNonZeroExit(t *testing.T) {
	t.Parallel()
	res, err := Run(context.Background(), "/bin/sh", "false", 0)
	if err == nil {
		t.Fatalf("expected error from false")
	}
	if res.ExitCode == 0 {
		t.Fatalf("exit code should be non-zero")
	}
}

func TestRunOutputCapTruncates(t *testing.T) {
	t.Parallel()
	cap := 64
	res, err := Run(context.Background(), "/bin/sh", "yes hello", cap)
	if err == nil {
		t.Fatalf("expected cap error")
	}
	if !res.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if len(res.Output) > cap {
		t.Fatalf("output length %d exceeds cap %d", len(res.Output), cap)
	}
}

func TestRunContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, "/bin/sh", "sleep 5", 0)
	if err == nil {
		t.Fatalf("expected error from cancelled run")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected err: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancel did not kill in time: %v", elapsed)
	}
}

func TestRunDefaultsShell(t *testing.T) {
	t.Parallel()
	res, err := Run(context.Background(), "", "printf ok", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("output = %q", res.Output)
	}
}

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
