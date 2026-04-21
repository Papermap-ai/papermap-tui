package teatest_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/teatest"
)

type fooMsg struct{ N int }
type barMsg struct{ S string }

func TestFindMsgInBatch(t *testing.T) {
	t.Parallel()

	cmd := tea.Batch(
		func() tea.Msg { return barMsg{S: "ignore"} },
		func() tea.Msg { return fooMsg{N: 42} },
	)

	got, ok := teatest.FindMsg[fooMsg](cmd)
	if !ok {
		t.Fatal("expected fooMsg in batch")
	}
	if got.N != 42 {
		t.Fatalf("got %+v", got)
	}
}

func TestFindMsgNested(t *testing.T) {
	t.Parallel()

	inner := tea.Batch(func() tea.Msg { return fooMsg{N: 7} })
	cmd := tea.Batch(
		func() tea.Msg { return barMsg{S: "noise"} },
		inner,
	)

	got, ok := teatest.FindMsg[fooMsg](cmd)
	if !ok || got.N != 7 {
		t.Fatalf("nested lookup failed: got=%+v ok=%v", got, ok)
	}
}

func TestFindMsgNilCmd(t *testing.T) {
	t.Parallel()

	if _, ok := teatest.FindMsg[fooMsg](nil); ok {
		t.Fatal("expected ok=false for nil cmd")
	}
}

func TestFindMsgMissing(t *testing.T) {
	t.Parallel()

	cmd := tea.Batch(func() tea.Msg { return barMsg{S: "only"} })
	if _, ok := teatest.FindMsg[fooMsg](cmd); ok {
		t.Fatal("expected ok=false when type absent")
	}
}
