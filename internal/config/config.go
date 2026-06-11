package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultAPIURL      = "https://prod.dataapi.papermap.ai"
	defaultFrontendURL = "https://papermap.ai"
)

type Config struct {
	APIURL string `yaml:"api_url"`
	// FrontendURL is the Papermap web app base URL used by the
	// browser-based login flow. The TUI opens
	// "<FrontendURL>/auth/login?cli_callback=...&state=..." to start
	// the flow. Empty means use the built-in default.
	FrontendURL string `yaml:"frontend_url,omitempty"`
	// SelectedModel is the LLM model slug (e.g. "gpt-5.4-mini",
	// "opus-4.6") the user picked via the model picker or TAB cycle.
	// Empty means defer to the backend default.
	SelectedModel string `yaml:"selected_model,omitempty"`
	// ShowThinking controls whether assistant reasoning traces render by
	// default. Missing or false means traces stay hidden until toggled on.
	ShowThinking bool `yaml:"show_thinking,omitempty"`
	// Shell controls the per-OS shell binary used by chat "!" mode.
	// Fields are scoped to the OS they apply to so unrelated
	// platforms ignore them rather than fight over a single key.
	Shell ShellConfig `yaml:"shell,omitempty"`
}

// ShellConfig holds per-OS shell preferences for chat "!" mode.
type ShellConfig struct {
	// Windows selects the shell family invoked on Windows. Valid
	// values: "pwsh" (PowerShell 7+, the default) and "cmd"
	// (cmd.exe). Both are resolved to absolute paths under
	// admin-only directories at runtime; we never honor PATH or
	// %COMSPEC%. Unset means default ("pwsh"). Ignored on non-Windows.
	Windows string `yaml:"windows,omitempty"`
}

// Default values for Shell config. Exposed as constants so the
// per-OS resolver in internal/app can reference them without a
// stringly-typed dependency.
const (
	ShellWindowsPwsh = "pwsh"
	ShellWindowsCmd  = "cmd"
)

func Default() Config {
	return Config{
		APIURL:      defaultAPIURL,
		FrontendURL: defaultFrontendURL,
		Shell:       ShellConfig{Windows: ShellWindowsPwsh},
	}
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

	if strings.TrimSpace(cfg.APIURL) == "" {
		cfg.APIURL = defaultAPIURL
	}

	if strings.TrimSpace(cfg.FrontendURL) == "" {
		cfg.FrontendURL = defaultFrontendURL
	}

	// Apply shell defaults after YAML load so a config file that
	// omits the section gets the same value as a brand-new install.
	if strings.TrimSpace(cfg.Shell.Windows) == "" {
		cfg.Shell.Windows = ShellWindowsPwsh
	}
	if err := validateShellWindows(cfg.Shell.Windows); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// validateShellWindows accepts only the documented enum values for
// shell.windows. Unknown values are rejected at load time so the user
// fixes a typo (e.g. "powershell" vs "pwsh") before the TUI starts.
func validateShellWindows(v string) error {
	switch v {
	case ShellWindowsPwsh, ShellWindowsCmd:
		return nil
	default:
		return fmt.Errorf("invalid shell.windows %q (want %q or %q)", v, ShellWindowsPwsh, ShellWindowsCmd)
	}
}

func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".papermap", "config.yaml"), nil
}

// Save writes the config to ~/.papermap/config.yaml atomically with 0o600
// permissions.
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

	// Strip the default URLs when persisting so the file stays minimal
	// and future default changes propagate to users who never customised
	// them. The PAPERMAP_*_URL env overrides are intentionally NOT
	// persisted; only fields explicitly set on cfg are written.
	persisted := cfg
	if strings.TrimSpace(persisted.APIURL) == defaultAPIURL {
		persisted.APIURL = ""
	}
	if strings.TrimSpace(persisted.FrontendURL) == defaultFrontendURL {
		persisted.FrontendURL = ""
	}
	if persisted.Shell.Windows == ShellWindowsPwsh {
		persisted.Shell.Windows = ""
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
