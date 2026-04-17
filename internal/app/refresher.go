package app

import (
	"context"
	"fmt"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
)

// refresherAdapter adapts the api.Client refresh flow to the auth.Refresher
// interface expected by the token store. It lives in the app package so the
// auth package does not need to import api.
type refresherAdapter struct {
	client *api.Client
	store  *auth.TokenStore
}

func newRefresher(client *api.Client, store *auth.TokenStore) *refresherAdapter {
	return &refresherAdapter{client: client, store: store}
}

// Refresh exchanges the given refresh token for a fresh set of credentials.
// It returns ready-to-save auth.Credentials derived from the backend response
// merged with any existing persisted state.
func (r *refresherAdapter) Refresh(ctx context.Context, refreshToken string) (auth.Credentials, error) {
	if r == nil || r.client == nil {
		return auth.Credentials{}, fmt.Errorf("refresher not ready")
	}

	tokens, err := r.client.Refresh(ctx, refreshToken)
	if err != nil {
		return auth.Credentials{}, err
	}

	// Use existing persisted creds as the baseline so we preserve user info
	// if the refresh response only returns tokens.
	existing, _ := r.store.Load()
	cred, err := tokens.ToCredentials(existing)
	if err != nil {
		return auth.Credentials{}, err
	}

	return cred, nil
}
