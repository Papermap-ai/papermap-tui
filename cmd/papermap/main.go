// Command papermap is the terminal UI entry point. It dispatches the
// `auth` subcommands and a small set of root flags before falling through
// to the Bubble Tea TUI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/papermap/papermap-tui/internal/app"
	cliauth "github.com/papermap/papermap-tui/internal/cli/auth"
	cliworkspace "github.com/papermap/papermap-tui/internal/cli/workspace"
)

// Build metadata. Populated by goreleaser via -ldflags
// `-X main.version=... -X main.commit=... -X main.date=...`. Defaults
// keep `go run` and `go install`-from-source builds working.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `Papermap - terminal UI for Papermap Data Platform 

Usage:
  papermap [flags]              Launch the interactive TUI
  papermap auth login           Sign in to Papermap (opens browser)
  papermap auth logout          Sign out and clear local credentials
  papermap auth whoami          Show the signed-in user
  papermap workspace create     Create a database-backed workspace
  papermap workspace list       List workspaces visible to the signed-in user

Flags:
  -v, --version               Print version and exit
  -u, --user                  Print the signed-in user and exit
      --api-url <url>         Override the Papermap API base URL for this run
  -h, --help                  Show this help message

Auth login flags:
      --browser               Sign in via browser (default)
      --password              Sign in via terminal email/password prompt
      --email <email>         Pre-fill the email field (used with --password)
      --frontend-url <url>    Override the Papermap web app base URL for this run

Environment:
  PAPERMAP_API_URL            Override the API base URL (same as --api-url)
  PAPERMAP_FRONTEND_URL       Override the frontend base URL (same as --frontend-url)
  PAPERMAP_FORCE_FILE_STORE   Force file-based credential storage

Run 'papermap auth login' before launching the TUI.
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "papermap: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	// Subcommand dispatch happens before flag parsing so subcommand flags
	// stay independent of root flags.
	if len(args) > 0 {
		switch args[0] {
		case "auth":
			return runAuth(args[1:], stdout, stderr)
		case "workspace":
			return runWorkspace(args[1:], stdout, stderr)
		case "logout":
			_, _ = fmt.Fprintln(stderr, "papermap: 'logout' is deprecated; use 'papermap auth logout'")
			return cliauth.RunLogout(context.Background(), stdout)
		case "help", "-h", "--help":
			_, _ = fmt.Fprint(stdout, usage)
			return nil
		}
	}

	fs := flag.NewFlagSet("papermap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { _, _ = fmt.Fprint(stderr, usage) }

	var (
		showVersion bool
		showUser    bool
		apiURL      string
	)
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.BoolVar(&showVersion, "v", false, "Print version and exit (shorthand)")
	fs.BoolVar(&showUser, "user", false, "Print the signed-in user and exit")
	fs.BoolVar(&showUser, "u", false, "Print the signed-in user and exit (shorthand)")
	fs.StringVar(&apiURL, "api-url", "", "Override the Papermap API base URL for this run")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	switch {
	case showVersion:
		_, _ = fmt.Fprintf(stdout, "papermap %s (commit %s, built %s)\n", version, commit, date)
		return nil
	case showUser:
		if err := cliauth.RunWhoami(stdout); err != nil {
			if errors.Is(err, cliauth.ErrNotSignedIn) {
				_, _ = fmt.Fprintln(stderr, "Not signed in. Run 'papermap auth login' to continue.")
				os.Exit(1)
			}
			return err
		}
		return nil
	}

	if apiURL != "" {
		// Honored by internal/config.Load via PAPERMAP_API_URL.
		_ = os.Setenv("PAPERMAP_API_URL", apiURL)
	}

	return app.Run()
}

func runAuth(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "papermap auth: missing subcommand (login | logout | whoami)")
		os.Exit(1)
	}

	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var (
			email       string
			apiURL      string
			frontendURL string
			useBrowser  bool
			usePassword bool
		)
		fs.StringVar(&email, "email", "", "Pre-fill the email field (used with --password)")
		fs.StringVar(&apiURL, "api-url", "", "Override the Papermap API base URL for this run")
		fs.StringVar(&frontendURL, "frontend-url", "", "Override the Papermap web app base URL for this run")
		fs.BoolVar(&useBrowser, "browser", false, "Sign in via browser (default)")
		fs.BoolVar(&usePassword, "password", false, "Sign in via terminal email/password prompt")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if useBrowser && usePassword {
			return errors.New("--browser and --password are mutually exclusive")
		}
		return cliauth.RunLogin(context.Background(), stdout, cliauth.LoginOptions{
			Email:               email,
			APIURLOverride:      apiURL,
			FrontendURLOverride: frontendURL,
			UseBrowser:          useBrowser,
			UsePassword:         usePassword,
		})
	case "logout":
		return cliauth.RunLogout(context.Background(), stdout)
	case "whoami":
		if err := cliauth.RunWhoami(stdout); err != nil {
			if errors.Is(err, cliauth.ErrNotSignedIn) {
				_, _ = fmt.Fprintln(stderr, "Not signed in. Run 'papermap auth login' to continue.")
				os.Exit(1)
			}
			return err
		}
		return nil
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, usage)
		return nil
	default:
		_, _ = fmt.Fprintf(stderr, "papermap auth: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
	return nil
}

func runWorkspace(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "papermap workspace: missing subcommand (create | list)")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("workspace create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var apiURL string
		fs.StringVar(&apiURL, "api-url", "", "Override the Papermap API base URL for this run")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if err := cliworkspace.RunCreate(context.Background(), stdout, cliworkspace.CreateOptions{
			APIURLOverride: apiURL,
		}); err != nil {
			if errors.Is(err, cliworkspace.ErrNotSignedIn) {
				_, _ = fmt.Fprintln(stderr, "Not signed in. Run 'papermap auth login' to continue.")
				os.Exit(1)
			}
			return err
		}
		return nil
	case "list":
		fs := flag.NewFlagSet("workspace list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var apiURL string
		fs.StringVar(&apiURL, "api-url", "", "Override the Papermap API base URL for this run")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if err := cliworkspace.RunList(context.Background(), stdout, cliworkspace.ListOptions{
			APIURLOverride: apiURL,
		}); err != nil {
			if errors.Is(err, cliworkspace.ErrNotSignedIn) {
				_, _ = fmt.Fprintln(stderr, "Not signed in. Run 'papermap auth login' to continue.")
				os.Exit(1)
			}
			return err
		}
		return nil
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, usage)
		return nil
	default:
		_, _ = fmt.Fprintf(stderr, "papermap workspace: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
	return nil
}
