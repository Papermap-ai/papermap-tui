package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"

	"github.com/papermap/papermap-tui/internal/api"
	authstore "github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/cli/clitheme"
	"github.com/papermap/papermap-tui/internal/config"
)

// LoginDeps lets tests inject fakes for the API client and credential
// store. Production callers use NewLoginDeps which wires the real ones.
type LoginDeps struct {
	Login    func(ctx context.Context, email, password string) (api.AuthTokens, error)
	Save     func(cred authstore.Credentials) error
	Warm     func(ctx context.Context) error // Optional unified-workspace warm-up.
	PromptIn io.Reader                       // Reserved; huh reads stdin directly.
}

// LoginOptions controls non-default behavior. Email may pre-fill the form.
type LoginOptions struct {
	Email          string
	APIURLOverride string
}

// RunLogin executes `papermap auth login`. It loads config, builds the api
// client + token store, prompts for credentials with huh, persists them,
// and warms the unified-workspace cache. Output is plain text on success
// or error.
func RunLogin(ctx context.Context, w io.Writer, opts LoginOptions) error {
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

	client, err := api.NewClient(cfg.APIURL, nil, store)
	if err != nil {
		return fmt.Errorf("build api client: %w", err)
	}

	deps := LoginDeps{
		Login: client.Login,
		Save:  store.Save,
		Warm: func(ctx context.Context) error {
			_, err := client.UnifiedWorkspace(ctx)
			return err
		},
	}

	return runLoginWith(ctx, w, deps, opts)
}

// runLoginWith is the testable core. It assumes deps are populated and
// only handles the form prompt + persistence flow.
func runLoginWith(ctx context.Context, w io.Writer, deps LoginDeps, opts LoginOptions) error {
	email := strings.TrimSpace(opts.Email)
	password := ""

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Sign in to Papermap").
				Description("Use your Papermap account to continue."),
			huh.NewInput().
				Title("Email").
				Placeholder("you@example.com").
				Value(&email).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("email is required")
					}
					if !strings.Contains(s, "@") {
						return errors.New("enter a valid email address")
					}
					return nil
				}),
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
		return fmt.Errorf("login form: %w", err)
	}
	if form.State == huh.StateAborted {
		return errors.New("login cancelled")
	}

	tokens, err := deps.Login(ctx, strings.TrimSpace(email), password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	cred, err := tokens.ToCredentials(authstore.Credentials{})
	if err != nil {
		return fmt.Errorf("decode credentials: %w", err)
	}

	if err := deps.Save(cred); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	// Warm-up is best-effort. Failure here should not block login success.
	if deps.Warm != nil {
		_ = deps.Warm(ctx)
	}

	display := strings.TrimSpace(cred.User.Email)
	if display == "" {
		display = strings.TrimSpace(email)
	}
	_, _ = fmt.Fprintf(w, "Signed in as %s.\n", display)
	return nil
}
