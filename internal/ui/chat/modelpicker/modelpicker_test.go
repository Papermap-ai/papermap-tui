package modelpicker_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat/modelpicker"
)

func sampleChoices() []api.ModelChoice {
	return []api.ModelChoice{
		{Slug: "gpt-5.4-mini", Display: "gpt-5.4-mini", Provider: "openai"},
		{Slug: "gpt-5.4", Display: "gpt-5.4", Provider: "openai"},
		{Slug: "sonnet-4.6", Display: "sonnet-4.6", Provider: "claude"},
		{Slug: "opus-4.6", Display: "opus-4.6", Provider: "claude"},
	}
}

func TestSetChoicesCursorLandsOnCurrent(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "sonnet-4.6")

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "current") {
		t.Fatalf("expected current marker rendered, got:\n%s", view)
	}
	// The cursor (›) should appear before sonnet-4.6 line — verify the
	// cursor line includes both the marker and the slug.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "sonnet-4.6") && strings.Contains(line, "›") {
			return
		}
	}
	t.Fatalf("expected cursor on sonnet-4.6 line, got:\n%s", view)
}

func TestNavigationCyclesChoices(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "gpt-5.4-mini")

	// Down twice should land on sonnet-4.6 (index 2).
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	updated, _ = updated.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	_, cmd := updated.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg, ok := cmd().(modelpicker.SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", cmd())
	}
	if msg.Model.Slug != "sonnet-4.6" {
		t.Fatalf("expected sonnet-4.6, got %q", msg.Model.Slug)
	}
}

func TestUpFromTopWrapsToBottom(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "gpt-5.4-mini")

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	_, cmd := updated.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg := cmd().(modelpicker.SelectMsg)
	if msg.Model.Slug != "opus-4.6" {
		t.Fatalf("expected wrap-around to opus-4.6, got %q", msg.Model.Slug)
	}
}

func TestEscEmitsCancel(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "gpt-5.4-mini")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected cancel cmd")
	}
	if _, ok := cmd().(modelpicker.CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestEnterWithNoChoicesIsNoop(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Fatalf("expected nil cmd on empty enter, got %T", cmd())
	}
}

func TestSingleChoiceShowsUpgradeHint(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices([]api.ModelChoice{
		{Slug: "gpt-5.4-mini", Display: "gpt-5.4-mini", Provider: "openai"},
	}, "gpt-5.4-mini")

	view := m.View(theme.Default(), 80)
	if !strings.Contains(view, "Upgrade your plan") {
		t.Fatalf("expected upgrade hint, got:\n%s", view)
	}
}

func TestProviderDividerRendersOncePerProvider(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "gpt-5.4-mini")

	view := m.View(theme.Default(), 80)
	if strings.Count(view, "OPENAI") != 1 {
		t.Fatalf("expected exactly one OPENAI divider, got:\n%s", view)
	}
	if strings.Count(view, "CLAUDE") != 1 {
		t.Fatalf("expected exactly one CLAUDE divider, got:\n%s", view)
	}
}

func TestNonKeyMsgIgnored(t *testing.T) {
	t.Parallel()

	m := modelpicker.NewModel()
	m.SetChoices(sampleChoices(), "gpt-5.4-mini")

	_, cmd := m.Update(struct{}{})
	if cmd != nil {
		t.Fatal("expected nil cmd for non-key msg")
	}
}
