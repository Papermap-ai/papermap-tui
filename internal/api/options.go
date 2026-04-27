package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

// FallbackDefaultModel is the slug used when the backend response carries
// no recommended default and no models. Mirrors the server-side default in
// papermap-da-api (app/utils/subscription_middleware.py:DEFAULT_LLM_MODEL).
const FallbackDefaultModel = "gpt-5.4-mini"

// ModelOptions is the decoded response of GET /api/v1/options/models.
// The endpoint returns a bare object (no {message,success,data} envelope).
// AllModels groups model slugs by provider; RecommendedModels exposes
// labelled defaults (e.g. {"Default": "gpt-5.4-mini", "Max": "opus-4.6"}).
type ModelOptions struct {
	AllModels         map[string][]string `json:"all_models"`
	RecommendedModels map[string]string   `json:"recommended_models"`
}

// ModelChoice is a flattened (provider, slug) pair suitable for picker
// rendering and TAB cycling. Display is the user-facing label (defaults
// to slug; callers can override).
type ModelChoice struct {
	Provider string
	Slug     string
	Display  string
}

// providerOrder defines the deterministic provider rendering order in
// the picker and TAB cycle. Unknown providers are appended alphabetically
// after the known list.
var providerOrder = []string{"openai", "claude", "google"}

// Flatten returns the available models in a stable order: providers in
// providerOrder first, then any unknown providers alphabetically; within
// each provider the slug order returned by the backend is preserved.
func (m ModelOptions) Flatten() []ModelChoice {
	if len(m.AllModels) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(providerOrder))
	ordered := make([]string, 0, len(m.AllModels))
	for _, p := range providerOrder {
		if _, ok := m.AllModels[p]; ok {
			ordered = append(ordered, p)
			seen[p] = struct{}{}
		}
	}

	extras := make([]string, 0)
	for p := range m.AllModels {
		if _, ok := seen[p]; ok {
			continue
		}
		extras = append(extras, p)
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)

	out := make([]ModelChoice, 0)
	for _, p := range ordered {
		for _, slug := range m.AllModels[p] {
			slug = strings.TrimSpace(slug)
			if slug == "" {
				continue
			}
			out = append(out, ModelChoice{
				Provider: p,
				Slug:     slug,
				Display:  slug,
			})
		}
	}
	return out
}

// DefaultSlug returns the canonical default model slug. Resolution order:
// recommended_models["Default"] -> first available slug -> FallbackDefaultModel.
func (m ModelOptions) DefaultSlug() string {
	if v, ok := m.RecommendedModels["Default"]; ok {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	for _, choice := range m.Flatten() {
		return choice.Slug
	}
	return FallbackDefaultModel
}

// HasSlug reports whether the given slug is present in any provider list.
func (m ModelOptions) HasSlug(slug string) bool {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false
	}
	for _, slugs := range m.AllModels {
		for _, s := range slugs {
			if s == slug {
				return true
			}
		}
	}
	return false
}

// ListModels fetches the available LLM model slugs for the current user.
// Auth is required; the server filters the response by subscription tier.
func (c *Client) ListModels(ctx context.Context) (ModelOptions, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/options/models", nil)
	if err != nil {
		return ModelOptions{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return ModelOptions{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[ModelOptions](resp)
}
