package app

import (
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

// modelTestModel returns a Model wired with a known set of choices and
// a clean HOME so persistSelectedModel does not hit the user's real disk.
func modelTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	m.availableModels = []api.ModelChoice{
		{Slug: "gpt-5.4-mini", Display: "gpt-5.4-mini", Provider: "openai"},
		{Slug: "sonnet-4.6", Display: "sonnet-4.6", Provider: "claude"},
		{Slug: "opus-4.6", Display: "opus-4.6", Provider: "claude"},
	}
	m.selectedModel = "gpt-5.4-mini"
	m.authenticated = true
	return m
}

func TestCycleModelAdvancesAndWraps(t *testing.T) {
	m := modelTestModel(t)

	m = m.cycleModel()
	if m.selectedModel != "sonnet-4.6" {
		t.Fatalf("after first cycle: got %q want sonnet-4.6", m.selectedModel)
	}
	m = m.cycleModel()
	if m.selectedModel != "opus-4.6" {
		t.Fatalf("after second cycle: got %q want opus-4.6", m.selectedModel)
	}
	m = m.cycleModel()
	if m.selectedModel != "gpt-5.4-mini" {
		t.Fatalf("expected wrap to first entry, got %q", m.selectedModel)
	}
}

func TestCycleModelNoopWhenSingleChoice(t *testing.T) {
	m := modelTestModel(t)
	m.availableModels = m.availableModels[:1]

	got := m.cycleModel()
	if got.selectedModel != m.selectedModel {
		t.Fatalf("expected single-choice cycle to be noop, got %q", got.selectedModel)
	}
}

func TestCycleModelPersistsSelection(t *testing.T) {
	m := modelTestModel(t)

	m = m.cycleModel()
	if m.config.SelectedModel != "sonnet-4.6" {
		t.Fatalf("expected config.SelectedModel updated to sonnet-4.6, got %q",
			m.config.SelectedModel)
	}
}

func TestSwitchModelReSelectingCurrentIsNoop(t *testing.T) {
	m := modelTestModel(t)
	m.screen = screenModelPicker

	got := m.switchModel(api.ModelChoice{Slug: "gpt-5.4-mini"})
	if got.selectedModel != "gpt-5.4-mini" {
		t.Fatalf("selectedModel changed unexpectedly: %q", got.selectedModel)
	}
	if got.screen != screenChat {
		t.Fatalf("expected screenChat after no-op switch, got %v", got.screen)
	}
}

func TestSwitchModelUpdatesAndPersists(t *testing.T) {
	m := modelTestModel(t)
	m.screen = screenModelPicker

	m = m.switchModel(api.ModelChoice{Slug: "opus-4.6", Display: "opus-4.6", Provider: "claude"})
	if m.selectedModel != "opus-4.6" {
		t.Fatalf("expected selectedModel=opus-4.6, got %q", m.selectedModel)
	}
	if m.screen != screenChat {
		t.Fatalf("expected screenChat after switch, got %v", m.screen)
	}
	if m.config.SelectedModel != "opus-4.6" {
		t.Fatalf("expected config persisted, got %q", m.config.SelectedModel)
	}
}

func TestOpenModelPickerSynthesizesEntryWhenEmpty(t *testing.T) {
	m := modelTestModel(t)
	m.availableModels = nil
	m.selectedModel = ""

	m.openModelPicker()
	if m.screen != screenModelPicker {
		t.Fatalf("expected screenModelPicker, got %v", m.screen)
	}
	view := m.modelPicker.View(m.theme, 80)
	if !strings.Contains(view, api.FallbackDefaultModel) {
		t.Fatalf("expected fallback model in picker view, got:\n%s", view)
	}
}

func TestModelDisplayNameFallsBackToSlug(t *testing.T) {
	m := modelTestModel(t)
	if got := m.modelDisplayName("unknown-slug"); got != "unknown-slug" {
		t.Fatalf("expected slug fallback, got %q", got)
	}
	if got := m.modelDisplayName(""); got != "" {
		t.Fatalf("expected empty for empty slug, got %q", got)
	}
}

func TestHydrateModelsFallbackChain(t *testing.T) {
	cases := []struct {
		name      string
		persisted string
		def       string
		models    []api.ModelChoice
		want      string
	}{
		{
			name:      "persisted wins when present",
			persisted: "sonnet-4.6",
			def:       "gpt-5.4-mini",
			models: []api.ModelChoice{
				{Slug: "gpt-5.4-mini"}, {Slug: "sonnet-4.6"},
			},
			want: "sonnet-4.6",
		},
		{
			name:      "falls to default when persisted unavailable",
			persisted: "stale-model",
			def:       "gpt-5.4-mini",
			models:    []api.ModelChoice{{Slug: "gpt-5.4-mini"}},
			want:      "gpt-5.4-mini",
		},
		{
			name:   "first available when no default",
			models: []api.ModelChoice{{Slug: "first"}, {Slug: "second"}},
			want:   "first",
		},
		{
			name: "synthesizes fallback when models empty",
			want: api.FallbackDefaultModel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			m, err := NewModel()
			if err != nil {
				t.Fatalf("NewModel: %v", err)
			}
			m.config.SelectedModel = tc.persisted
			m.hydrateModels(tc.models, tc.def)
			if m.selectedModel != tc.want {
				t.Fatalf("selectedModel = %q want %q", m.selectedModel, tc.want)
			}
		})
	}
}
