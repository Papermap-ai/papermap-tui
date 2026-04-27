package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtripPersistsSelectedModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(apiURLEnvKey, "")

	cfg := Config{
		APIURL:        "https://custom.example",
		SelectedModel: "opus-4.6",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.APIURL != cfg.APIURL {
		t.Fatalf("APIURL: got %q want %q", got.APIURL, cfg.APIURL)
	}
	if got.SelectedModel != cfg.SelectedModel {
		t.Fatalf("SelectedModel: got %q want %q", got.SelectedModel, cfg.SelectedModel)
	}
}

func TestSaveOmitsSelectedModelWhenEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(apiURLEnvKey, "")

	if err := Save(Config{APIURL: "https://custom.example"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".papermap", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if contains(string(data), "selected_model") {
		t.Fatalf("expected selected_model to be omitted, got:\n%s", data)
	}
}

func TestSaveStripsDefaultAPIURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(apiURLEnvKey, "")

	if err := Save(Config{APIURL: defaultAPIURL, SelectedModel: "x"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".papermap", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if contains(string(data), defaultAPIURL) {
		t.Fatalf("expected default URL to be stripped, got:\n%s", data)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.APIURL != defaultAPIURL {
		t.Fatalf("Load should fall back to default, got %q", got.APIURL)
	}
	if got.SelectedModel != "x" {
		t.Fatalf("SelectedModel lost: got %q", got.SelectedModel)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
