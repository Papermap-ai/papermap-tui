package api

import (
	"context"
	"fmt"

	"github.com/papermap/papermap-tui/internal/auth"
)

// refresherAdapter adapts (*Client).Refresh to the auth.Refresher
// interface expected by *auth.TokenStore. It is exported via
// NewRefresher so the TUI and CLI subcommands can share one wiring
// without importing each other.
type refresherAdapter struct {
	client *Client
}

// NewRefresher returns an auth.Refresher backed by the given client. Wire the
// result into a token store so AccessToken can lazily refresh expired tokens.
func NewRefresher(client *Client) auth.Refresher {
	return &refresherAdapter{client: client}
}

// Refresh exchanges the given refresh token for a fresh set of
// credentials. The returned auth.Credentials preserves any existing user
// info from disk if the refresh response only returns tokens.
func (r *refresherAdapter) Refresh(ctx context.Context, refreshToken string) (auth.Credentials, error) {
	if r == nil || r.client == nil {
		return auth.Credentials{}, fmt.Errorf("refresher not ready")
	}

	tokens, err := r.client.Refresh(ctx, refreshToken)
	if err != nil {
		return auth.Credentials{}, err
	}

	cred, err := tokens.ToCredentials(auth.Credentials{})
	if err != nil {
		return auth.Credentials{}, err
	}

	return cred, nil
}
