package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/ui/components/palette"
)

func TestStartupAppliesShowThinkingFromConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}

	updated, _ := m.handleStartup(startupMsg{
		config: config.Config{ShowThinking: true},
	})
	got := updated.(Model)
	if !got.chat.ShowThinking() {
		t.Fatal("chat ShowThinking = false, want true from config")
	}
}

func TestCtrlTPersistsShowThinking(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.screen = screenChat
	m.authenticated = true
	m.config = config.Default()
	m.chat.SetShowThinking(false)

	updated, _ := m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	got := updated.(Model)
	if !got.chat.ShowThinking() {
		t.Fatal("chat ShowThinking = false after ctrl+t, want true")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.ShowThinking {
		t.Fatal("persisted ShowThinking = false, want true")
	}

	updated, _ = got.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	got = updated.(Model)
	if got.chat.ShowThinking() {
		t.Fatal("chat ShowThinking = true after second ctrl+t, want false")
	}

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ShowThinking {
		t.Fatal("persisted ShowThinking = true after second ctrl+t, want false")
	}
}

func TestPaletteToggleThinkingPersistsShowThinking(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.screen = screenCommandPalette
	m.config = config.Default()
	m.chat.SetShowThinking(false)

	cmd := m.dispatchPaletteCommand(palette.Command{ID: commandToggleThinking})
	if cmd != nil {
		t.Fatal("toggle thinking returned unexpected command")
	}
	if !m.chat.ShowThinking() {
		t.Fatal("chat ShowThinking = false after palette toggle, want true")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.ShowThinking {
		t.Fatal("persisted ShowThinking = false, want true")
	}
}
