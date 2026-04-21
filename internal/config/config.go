package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultAPIURL = "https://dataapi.papermap.ai"
	apiURLEnvKey  = "PAPERMAP_API_URL"
)

type Config struct {
	APIURL string `yaml:"api_url"`
}

func Default() Config {
	return Config{APIURL: defaultAPIURL}
}

func Load() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}

	return LoadFromPaths(filepath.Join(homeDir, ".papermap", "config.yaml"))
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
