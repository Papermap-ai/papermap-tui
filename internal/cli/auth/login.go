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

// LoginOptions controls non-default behavior. Email may pre-fill the
// form (only used by the password flow). UseBrowser forces the
// browser-based flow; UsePassword forces the legacy email/password
// flow. If neither is set, the browser flow is used.
type LoginOptions struct {
	Email               string
	APIURLOverride      string
	FrontendURLOverride string
	UseBrowser          bool
	UsePassword         bool
}

// RunLogin executes `papermap auth login`. By default it runs the
// browser-based flow; pass UsePassword to fall back to the legacy
// terminal email/password prompts (useful for SSH or headless
// environments). If the credential store already holds valid (or
// refreshable) credentials, the flow short-circuits with a friendly
// "already signed in" message.
func RunLogin(ctx context.Context, w io.Writer, opts LoginOptions) error {
	if email, ok := alreadySignedIn(ctx, opts); ok {
		_, _ = fmt.Fprintf(w, "Already signed in as %s.\n", email)
		return nil
	}
	if opts.UsePassword {
		return runPasswordLogin(ctx, w, opts)
	}
	return RunBrowserLogin(ctx, w, BrowserLoginOptions{
		APIURLOverride:      opts.APIURLOverride,
		FrontendURLOverride: opts.FrontendURLOverride,
	})
}

// alreadySignedIn reports whether the credential store currently holds
// usable credentials for the configured API. It attempts a token
// refresh if the access token has expired. On any error or missing
// credentials it returns false so the caller can run a fresh login.
// The returned string is a best-effort display name (email if known,
// otherwise "your Papermap account").
func alreadySignedIn(ctx context.Context, opts LoginOptions) (string, bool) {
	cfg, err := config.Load()
	if err != nil {
		return "", false
	}
	if v := strings.TrimSpace(opts.APIURLOverride); v != "" {
		cfg.APIURL = v
	}

	store, err := authstore.DefaultStore()
	if err != nil {
		return "", false
	}

	cred, err := store.Load()
	if err != nil {
		return "", false
	}

	client, err := api.NewClient(cfg.APIURL, nil, store)
	if err != nil {
		return "", false
	}
	store.SetRefresher(api.NewRefresher(client, store))

	token, err := store.AccessToken(ctx)
	if err != nil || token == "" {
		return "", false
	}

	// Reload in case AccessToken refreshed and updated the user record.
	if updated, lerr := store.Load(); lerr == nil {
		cred = updated
	}

	display := strings.TrimSpace(cred.User.Email)
	if display == "" {
		display = "your Papermap account"
	}
	return display, true
}

// runPasswordLogin loads config, builds the api client + token store,
// prompts for credentials with huh, persists them, and warms the
// unified-workspace cache. Output is plain text on success or error.
func runPasswordLogin(ctx context.Context, w io.Writer, opts LoginOptions) error {
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
