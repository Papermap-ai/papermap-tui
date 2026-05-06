package app

import (
	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
)

// newRefresher is a thin alias kept so existing app code does not need to
// change. The implementation lives in the api package so CLI subcommands
// can reuse it without importing app.
func newRefresher(client *api.Client, store *auth.TokenStore) auth.Refresher {
	return api.NewRefresher(client, store)
}
