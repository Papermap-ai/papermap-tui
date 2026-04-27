package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultAPIURL = "https://dev.dataapi.papermap.ai"
	apiURLEnvKey  = "PAPERMAP_API_URL"
)

type Config struct {
	APIURL string `yaml:"api_url"`
	// SelectedModel is the LLM model slug (e.g. "gpt-5.4-mini",
	// "opus-4.6") the user picked via the model picker or TAB cycle.
	// Empty means defer to the backend default.
	SelectedModel string `yaml:"selected_model,omitempty"`
}

func Default() Config {
	return Config{APIURL: defaultAPIURL}
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFromPaths(path)
}

func LoadFromPaths(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("read config file: %w", err)
		}
	} else if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config file: %w", err)
		}
	}

	if envURL := strings.TrimSpace(os.Getenv(apiURLEnvKey)); envURL != "" {
		cfg.APIURL = envURL
	}

	if strings.TrimSpace(cfg.APIURL) == "" {
		cfg.APIURL = defaultAPIURL
	}

	return cfg, nil
}

func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".papermap", "config.yaml"), nil
}

// Save writes the config to ~/.papermap/config.yaml atomically with 0o600
// permissions. The PAPERMAP_API_URL env override is intentionally NOT
// persisted; only fields explicitly set on cfg are written.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return saveConfigTo(path, cfg)
}

func saveConfigTo(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Strip the default URL when persisting so the file stays minimal and
	// future default changes propagate to users who never customised it.
	persisted := cfg
	if strings.TrimSpace(persisted.APIURL) == defaultAPIURL {
		persisted.APIURL = ""
	}

	payload, err := yaml.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config: %w", err)
	}

	return nil
}
