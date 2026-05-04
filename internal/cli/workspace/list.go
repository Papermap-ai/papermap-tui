package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/papermap/papermap-tui/internal/api"
	authstore "github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
)

// ListOptions controls non-default behavior.
type ListOptions struct {
	APIURLOverride string
}

// ListDeps lets tests inject a fake list call.
type ListDeps struct {
	List func(ctx context.Context) ([]api.WorkspaceSummary, error)
}

// RunList executes `papermap workspace list`. It paginates the user's
// workspaces and renders them as a tab-aligned table. The local cache is
// also refreshed as a side effect.
func RunList(ctx context.Context, w io.Writer, opts ListOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if strings.TrimSpace(opts.APIURLOverride) != "" {
		cfg.APIURL = strings.TrimSpace(opts.APIURLOverride)
	}

	store, err := authstore.DefaultStore()
	if err != nil {
		return fmt.Errorf("init credential store: %w", err)
	}

	if _, err := store.Load(); err != nil {
		if errors.Is(err, authstore.ErrNoCredentials) {
			return ErrNotSignedIn
		}
		return fmt.Errorf("load stored credentials: %w", err)
	}

	client, err := api.NewClient(cfg.APIURL, nil, store)
	if err != nil {
		return fmt.Errorf("build api client: %w", err)
	}
	store.SetRefresher(api.NewRefresher(client, store))

	return runListWith(ctx, w, ListDeps{List: client.ListWorkspaces})
}

func runListWith(ctx context.Context, w io.Writer, deps ListDeps) error {
	summaries, err := deps.List(ctx)
	if err != nil {
		if errors.Is(err, authstore.ErrSessionExpired) {
			return ErrNotSignedIn
		}
		return fmt.Errorf("list workspaces: %w", err)
	}

	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(w, "No workspaces yet. Run 'papermap workspace create' to add one.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTYPE\tUNIFIED")
	for _, s := range summaries {
		name := s.Name
		if strings.TrimSpace(name) == "" {
			name = "(unnamed)"
		}
		wsType := s.WorkspaceType
		if strings.TrimSpace(wsType) == "" {
			wsType = "-"
		}
		unified := "no"
		if s.IsUnified {
			unified = "yes"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.WorkspaceID, name, wsType, unified)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("render workspace table: %w", err)
	}

	// Refresh the cache opportunistically. Failures are not user-facing
	// because the list call already succeeded.
	_ = saveWorkspaceCache(summaries)

	return nil
}
