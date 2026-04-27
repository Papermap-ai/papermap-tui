package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestModelOptionsFlattenOrdering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   api.ModelOptions
		want []string
	}{
		{
			name: "known providers in fixed order",
			in: api.ModelOptions{
				AllModels: map[string][]string{
					"google": {"gemini-3-pro", "gemini-3-flash"},
					"openai": {"gpt-5.4", "gpt-5.4-mini"},
					"claude": {"sonnet-4.6"},
				},
			},
			want: []string{
				"gpt-5.4", "gpt-5.4-mini",
				"sonnet-4.6",
				"gemini-3-pro", "gemini-3-flash",
			},
		},
		{
			name: "unknown providers appended alphabetically",
			in: api.ModelOptions{
				AllModels: map[string][]string{
					"openai":  {"gpt-5.4-mini"},
					"zeta":    {"zeta-1"},
					"alpha":   {"alpha-1"},
					"unknown": {"u-1"},
				},
			},
			want: []string{"gpt-5.4-mini", "alpha-1", "u-1", "zeta-1"},
		},
		{
			name: "blank slugs are skipped",
			in: api.ModelOptions{
				AllModels: map[string][]string{
					"openai": {"", "gpt-5.4-mini", "  "},
				},
			},
			want: []string{"gpt-5.4-mini"},
		},
		{
			name: "empty map returns nil",
			in:   api.ModelOptions{},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.in.Flatten()
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			slugs := make([]string, len(got))
			for i, c := range got {
				slugs[i] = c.Slug
			}
			if !reflect.DeepEqual(slugs, tc.want) {
				t.Fatalf("flatten order mismatch: got %v want %v", slugs, tc.want)
			}
		})
	}
}

func TestModelOptionsDefaultSlugFallbackChain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   api.ModelOptions
		want string
	}{
		{
			name: "recommended default wins",
			in: api.ModelOptions{
				AllModels:         map[string][]string{"openai": {"gpt-5.4"}},
				RecommendedModels: map[string]string{"Default": "opus-4.6"},
			},
			want: "opus-4.6",
		},
		{
			name: "first available when no recommended default",
			in: api.ModelOptions{
				AllModels: map[string][]string{"openai": {"gpt-5.4-mini"}},
			},
			want: "gpt-5.4-mini",
		},
		{
			name: "fallback constant when nothing available",
			in:   api.ModelOptions{},
			want: api.FallbackDefaultModel,
		},
		{
			name: "blank recommended falls through to first",
			in: api.ModelOptions{
				AllModels:         map[string][]string{"openai": {"gpt-5.4-mini"}},
				RecommendedModels: map[string]string{"Default": "  "},
			},
			want: "gpt-5.4-mini",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.DefaultSlug(); got != tc.want {
				t.Fatalf("DefaultSlug = %q want %q", got, tc.want)
			}
		})
	}
}

func TestModelOptionsHasSlug(t *testing.T) {
	t.Parallel()

	opts := api.ModelOptions{
		AllModels: map[string][]string{
			"openai": {"gpt-5.4-mini"},
			"claude": {"sonnet-4.6"},
		},
	}

	if !opts.HasSlug("sonnet-4.6") {
		t.Fatal("expected HasSlug to find sonnet-4.6")
	}
	if opts.HasSlug("nonexistent") {
		t.Fatal("expected HasSlug to reject unknown slug")
	}
	if opts.HasSlug("") {
		t.Fatal("expected HasSlug to reject empty slug")
	}
}

func TestListModelsDecodesBareResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/options/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		// Bare object, no {message,success,data} envelope.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"all_models": map[string]any{
				"openai": []string{"gpt-5.4-mini", "gpt-5.4"},
				"claude": []string{"sonnet-4.6"},
			},
			"recommended_models": map[string]any{
				"Default": "gpt-5.4-mini",
				"Max":     "sonnet-4.6",
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	opts, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if got := opts.DefaultSlug(); got != "gpt-5.4-mini" {
		t.Fatalf("DefaultSlug = %q want gpt-5.4-mini", got)
	}
	if !opts.HasSlug("sonnet-4.6") {
		t.Fatal("expected sonnet-4.6 to be present")
	}
}

func TestInsightRequestLLMModelOmitempty(t *testing.T) {
	t.Parallel()

	t.Run("omitted when empty", func(t *testing.T) {
		t.Parallel()
		body, err := json.Marshal(api.InsightRequest{Prompt: "hi"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := raw["llm_model"]; ok {
			t.Fatalf("expected llm_model to be omitted, got %v", raw)
		}
	})

	t.Run("included when set", func(t *testing.T) {
		t.Parallel()
		body, err := json.Marshal(api.InsightRequest{Prompt: "hi", LLMModel: "opus-4.6"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if raw["llm_model"] != "opus-4.6" {
			t.Fatalf("expected llm_model=opus-4.6, got %v", raw["llm_model"])
		}
	})
}

// Compile-time guard to keep the io import used when other tests in
// the package change.
var _ = io.Discard
