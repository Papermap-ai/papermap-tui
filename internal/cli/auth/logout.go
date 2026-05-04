package auth

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/papermap/papermap-tui/internal/api"
	authstore "github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
)

// RunLogout executes `papermap auth logout`. It revokes the session
// remotely when possible, then clears local credentials and the workspace
// cache. Output is a one-line status message.
func RunLogout(ctx context.Context, w io.Writer) error {
	store, err := authstore.DefaultStore()
	if err != nil {
		return fmt.Errorf("init credential store: %w", err)
	}

	cred, err := store.Load()
	switch {
	case err == nil:
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		client, err := api.NewClient(cfg.APIURL, nil, store)
		if err != nil {
			return fmt.Errorf("build api client: %w", err)
		}
		store.SetRefresher(api.NewRefresher(client, store))

		if cred.AccessToken != "" {
			if err := client.Logout(ctx); err != nil {
				if clearErr := store.Clear(); clearErr != nil {
					return fmt.Errorf("clear credentials: %w", clearErr)
				}
				_, _ = fmt.Fprintln(w, "Logged out locally. Remote logout failed.")
				return nil
			}
		}

		if err := store.Clear(); err != nil {
			return fmt.Errorf("clear credentials: %w", err)
		}
		_ = config.ClearWorkspaces()
		clearSelectedModel(cfg)
		_, _ = fmt.Fprintln(w, "Logged out successfully.")
		return nil

	case errors.Is(err, authstore.ErrNoCredentials):
		_, _ = fmt.Fprintln(w, "No local session found.")
		return nil

	default:
		return fmt.Errorf("load stored credentials: %w", err)
	}
}

// clearSelectedModel removes any persisted model selection so a fresh
// login starts on the backend default. Errors are intentionally swallowed
// because logout has already succeeded; surfacing a config-write error
// would only confuse the user.
func clearSelectedModel(cfg config.Config) {
	if cfg.SelectedModel == "" {
		return
	}
	cfg.SelectedModel = ""
	_ = config.Save(cfg)
}
