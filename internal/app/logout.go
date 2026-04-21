package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
)

func Logout() (string, error) {
	store, err := auth.DefaultStore()
	if err != nil {
		return "", err
	}

	cred, err := store.Load()
	switch {
	case err == nil:
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}

		client, err := api.NewClient(cfg.APIURL, nil, store)
		if err != nil {
			return "", err
		}

		if cred.AccessToken != "" {
			if err := client.Logout(context.Background()); err != nil {
				if clearErr := store.Clear(); clearErr != nil {
					return "", clearErr
				}

				return "Logged out locally. Remote logout failed.", nil
			}
		}

		if err := store.Clear(); err != nil {
			return "", err
		}

		_ = config.ClearWorkspaces()

		return "Logged out successfully.", nil

	case errors.Is(err, auth.ErrNoCredentials):
		return "No local session found.", nil

	default:
		return "", fmt.Errorf("load stored credentials: %w", err)
	}
}
