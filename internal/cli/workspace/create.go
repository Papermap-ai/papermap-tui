// Package workspace implements the `papermap workspace` CLI subcommands:
// create and list. Each command is a one-shot runner that loads
// credentials, builds an API client, and either prompts the user with a
// huh form (create) or paginates the workspace list (list).
package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"github.com/papermap/papermap-tui/internal/api"
	authstore "github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/cli/clitheme"
	"github.com/papermap/papermap-tui/internal/config"
)

// ErrNotSignedIn is returned when there are no stored credentials. Main
// translates this to a friendly stderr message + non-zero exit code.
var ErrNotSignedIn = errors.New("not signed in")

// CreateOptions controls non-default behavior. APIURLOverride mirrors the
// `--api-url` flag accepted by the auth subcommands.
type CreateOptions struct {
	APIURLOverride string
}

// CreateDeps lets tests inject fakes for the API client. Production
// callers use RunCreate which wires the real client.
type CreateDeps struct {
	Create  func(ctx context.Context, req api.CreateWorkspaceRequest) (*api.WorkspaceSummary, error)
	Refresh func(ctx context.Context) error
}

// dbType holds metadata for one supported database backend.
type dbType struct {
	label       string // Display label in the select.
	enum        string // Server-side enum value (uppercase).
	defaultPort int    // Used when the user leaves the port blank.
}

var supportedDBTypes = []dbType{
	{label: "Postgres", enum: "POSTGRES", defaultPort: 5432},
	{label: "MySQL", enum: "MYSQL", defaultPort: 3306},
	{label: "MongoDB", enum: "MONGODB", defaultPort: 27017},
	{label: "Supabase", enum: "SUPABASE", defaultPort: 5432},
}

// RunCreate executes `papermap workspace create`. It loads config + the
// stored credential, builds an API client, prompts for workspace details,
// posts to the backend, and refreshes the local workspace cache so the
// TUI picks up the new entry on next launch.
func RunCreate(ctx context.Context, w io.Writer, opts CreateOptions) error {
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

	deps := CreateDeps{
		Create: client.CreateWorkspace,
		Refresh: func(ctx context.Context) error {
			summaries, err := client.ListWorkspaces(ctx)
			if err != nil {
				return err
			}
			return saveWorkspaceCache(summaries)
		},
	}

	return runCreateWith(ctx, w, deps)
}

// runCreateWith is the testable core. It assumes deps are populated.
func runCreateWith(ctx context.Context, w io.Writer, deps CreateDeps) error {
	var (
		name     string
		dbEnum   = supportedDBTypes[0].enum
		host     string
		portStr  string
		dbName   string
		username string
		password string
	)

	options := make([]huh.Option[string], 0, len(supportedDBTypes))
	for _, t := range supportedDBTypes {
		options = append(options, huh.NewOption(t.label, t.enum))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Create a workspace").
				Description("Connect Papermap to a database. Postgres, MySQL, MongoDB, and Supabase are supported."),
			huh.NewInput().
				Title("Workspace name").
				Placeholder("Acme Production DB").
				Value(&name).
				Validate(requiredString("workspace name", 255)),
			huh.NewSelect[string]().
				Title("Database type").
				Options(options...).
				Value(&dbEnum),
			huh.NewInput().
				Title("Host").
				Placeholder("db.example.com").
				Value(&host).
				Validate(requiredString("host", 0)),
			huh.NewInput().
				Title("Port").
				Description("Leave blank to use the default for the selected database type.").
				Value(&portStr).
				Validate(optionalPort),
			huh.NewInput().
				Title("Database name").
				Placeholder("postgres").
				Value(&dbName).
				Validate(requiredString("database name", 0)),
			huh.NewInput().
				Title("Username").
				Value(&username).
				Validate(requiredString("username", 0)),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("password is required")
					}
					return nil
				}),
		),
	).WithTheme(clitheme.PapermapHuh())

	if err := form.Run(); err != nil {
		return fmt.Errorf("create form: %w", err)
	}
	if form.State == huh.StateAborted {
		return errors.New("create cancelled")
	}

	port, err := resolvePort(portStr, dbEnum)
	if err != nil {
		return err
	}

	dbInput := DatabaseInputFromForm(dbEnum, host, port, dbName, username, password)
	req := api.CreateWorkspaceRequest{
		Name:     strings.TrimSpace(name),
		Database: &dbInput,
	}

	created, err := deps.Create(ctx, req)
	if err != nil {
		if errors.Is(err, authstore.ErrSessionExpired) {
			return ErrNotSignedIn
		}
		return fmt.Errorf("create workspace: %w", err)
	}

	displayName := strings.TrimSpace(req.Name)
	if created != nil && strings.TrimSpace(created.Name) != "" {
		displayName = created.Name
	}

	if created != nil && strings.TrimSpace(created.WorkspaceID) != "" {
		_, _ = fmt.Fprintf(w, "Workspace %q created (id: %s).\n", displayName, created.WorkspaceID)
	} else {
		_, _ = fmt.Fprintf(w, "Workspace %q created.\n", displayName)
	}
	_, _ = fmt.Fprintln(w, "Verifying connection in background.")

	// Refresh the local workspace cache so the TUI sees the new
	// workspace immediately. Failures are best-effort; clear the cache
	// instead so the next TUI launch refetches.
	if deps.Refresh != nil {
		if err := deps.Refresh(ctx); err != nil {
			_ = config.ClearWorkspaces()
		}
	}

	return nil
}

// DatabaseInputFromForm builds an api.DatabaseInput by value. The
// indirection keeps runCreateWith easy to read and lets tests assert on
// the assembled struct.
func DatabaseInputFromForm(enum, host string, port int, dbName, username, password string) api.DatabaseInput {
	return api.DatabaseInput{
		DatabaseType: enum,
		Host:         strings.TrimSpace(host),
		Port:         port,
		Name:         strings.TrimSpace(dbName),
		UserName:     strings.TrimSpace(username),
		Password:     password,
	}
}

func requiredString(label string, maxLen int) func(string) error {
	return func(s string) error {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return fmt.Errorf("%s is required", label)
		}
		if maxLen > 0 && len(trimmed) > maxLen {
			return fmt.Errorf("%s must be %d characters or fewer", label, maxLen)
		}
		return nil
	}
}

func optionalPort(s string) error {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return errors.New("port must be a number")
	}
	if port <= 0 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func resolvePort(input, enum string) (int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed != "" {
		port, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("invalid port %q", trimmed)
		}
		return port, nil
	}
	for _, t := range supportedDBTypes {
		if t.enum == enum {
			return t.defaultPort, nil
		}
	}
	return 0, fmt.Errorf("unknown database type %q", enum)
}

// saveWorkspaceCache converts API summaries into the on-disk cache shape
// and writes them via config.SaveWorkspaces.
func saveWorkspaceCache(summaries []api.WorkspaceSummary) error {
	entries := make([]config.WorkspaceEntry, 0, len(summaries))
	for _, s := range summaries {
		entries = append(entries, config.WorkspaceEntry{
			WorkspaceID:      s.WorkspaceID,
			Name:             s.Name,
			WorkspaceType:    s.WorkspaceType,
			IsUnified:        s.IsUnified,
			DefaultDashboard: s.DefaultDashboard,
		})
	}
	return config.SaveWorkspaces(config.WorkspaceCache{Workspaces: entries})
}
