package auth

import (
	"errors"
	"fmt"
	"io"
	"strings"

	authstore "github.com/papermap/papermap-tui/internal/auth"
)

// ErrNotSignedIn is returned by Whoami when no credentials are stored.
// Callers (main.go) translate this to a non-zero exit code.
var ErrNotSignedIn = errors.New("not signed in")

// RunWhoami prints the email (and full name if available) of the
// currently signed-in user. Returns ErrNotSignedIn when there is no
// stored session.
func RunWhoami(w io.Writer) error {
	store, err := authstore.DefaultStore()
	if err != nil {
		return fmt.Errorf("init credential store: %w", err)
	}

	cred, err := store.Load()
	switch {
	case err == nil:
		fmt.Fprintln(w, formatUser(cred.User))
		return nil
	case errors.Is(err, authstore.ErrNoCredentials):
		return ErrNotSignedIn
	default:
		return fmt.Errorf("load credentials: %w", err)
	}
}

func formatUser(u authstore.User) string {
	email := strings.TrimSpace(u.Email)
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if email == "" && name == "" {
		return "(unknown user)"
	}
	if name == "" {
		return email
	}
	if email == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", email, name)
}
