//go:build unix

package shell

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunSuccess(t *testing.T) {
	t.Parallel()
	res, err := Run(context.Background(), "/bin/sh", "printf hello", 0, nil)
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
	res, err := Run(context.Background(), "/bin/sh", "false", 0, nil)
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
	res, err := Run(context.Background(), "/bin/sh", "yes hello", cap, nil)
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
	_, err := Run(ctx, "/bin/sh", "sleep 5", 0, nil)
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
	res, err := Run(context.Background(), "", "printf ok", 0, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("output = %q", res.Output)
	}
}
