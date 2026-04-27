package app

import (
	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/ui/chat/modelpicker"
)

// openModelPicker primes the picker with the available choices and switches
// to the picker screen. Falls back to a synthesized single-entry list when
// no fetch has populated availableModels yet.
func (m *Model) openModelPicker() {
	choices := m.availableModels
	if len(choices) == 0 {
		fallback := m.selectedModel
		if fallback == "" {
			fallback = api.FallbackDefaultModel
		}
		choices = []api.ModelChoice{{
			Provider: "openai",
			Slug:     fallback,
			Display:  fallback,
		}}
	}
	m.modelPicker.SetChoices(choices, m.selectedModel)
	m.screen = screenModelPicker
}

// switchModel applies the picker selection: updates the badge, persists
// the slug, and routes back to chat. Re-selecting the active model is a
// no-op beyond closing the picker.
func (m Model) switchModel(choice api.ModelChoice) Model {
	if choice.Slug == "" || choice.Slug == m.selectedModel {
		m.screen = screenChat
		return m
	}
	m.selectedModel = choice.Slug
	m.chat.SetModel(m.modelDisplayName(choice.Slug))
	m.persistSelectedModel()
	m.screen = screenChat
	return m
}

// cycleModel advances selectedModel to the next entry in availableModels,
// wrapping around the end. Used by the TAB shortcut to flip through
// models without opening the picker.
func (m Model) cycleModel() Model {
	if len(m.availableModels) <= 1 {
		return m
	}
	idx := 0
	for i, c := range m.availableModels {
		if c.Slug == m.selectedModel {
			idx = i
			break
		}
	}
	next := m.availableModels[(idx+1)%len(m.availableModels)]
	m.selectedModel = next.Slug
	m.chat.SetModel(m.modelDisplayName(next.Slug))
	m.persistSelectedModel()
	return m
}

// overlayModelPicker composites the picker modal centered on the chat view.
func (m Model) overlayModelPicker(base string) string {
	return m.centerOverlay(base, m.modelPicker.View(m.theme, m.width))
}

// handleModelPickerSelect routes the picker's SelectMsg.
func (m Model) handleModelPickerSelect(msg modelpicker.SelectMsg) Model {
	return m.switchModel(msg.Model)
}

// handleModelPickerCancel routes the picker's CancelMsg.
func (m Model) handleModelPickerCancel() Model {
	if m.authenticated {
		m.screen = screenChat
	} else {
		m.screen = screenLanding
	}
	return m
}
