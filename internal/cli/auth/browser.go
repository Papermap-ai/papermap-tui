// Package auth implements the `papermap auth` CLI subcommands. This file
// provides the browser-based login flow: a one-shot localhost callback
// server receives a short-lived authorization code minted by the Papermap
// frontend, then exchanges it for normal access and refresh tokens via
// the data API. No tokens ever travel through the browser URL.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/papermap/papermap-tui/internal/api"
	authstore "github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
)

// browserLoginTimeout bounds the entire flow. If the user abandons the
// browser tab the callback server gets torn down and the CLI errors.
const browserLoginTimeout = 2 * time.Minute

// callbackPath is the only path the local server listens on. The
// frontend must redirect to "<callback_url>/callback" after minting the
// CLI code.
const callbackPath = "/callback"

// BrowserLoginOptions controls the browser-login flow. APIURLOverride
// and FrontendURLOverride are only honored if non-empty after trimming.
type BrowserLoginOptions struct {
	APIURLOverride      string
	FrontendURLOverride string
}

// callbackResult is delivered exactly once by the local callback server.
// On success code is set; on failure err is set.
type callbackResult struct {
	code string
	err  error
}

// RunBrowserLogin executes `papermap auth login` with browser flow. It
// boots a localhost callback server, opens the Papermap web app login
// page, waits for the frontend to redirect back with a one-time code,
// then exchanges that code for tokens via the data API and persists
// them using the existing credential store.
func RunBrowserLogin(ctx context.Context, w io.Writer, opts BrowserLoginOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if v := strings.TrimSpace(opts.APIURLOverride); v != "" {
		cfg.APIURL = v
	}
	if v := strings.TrimSpace(opts.FrontendURLOverride); v != "" {
		cfg.FrontendURL = v
	}

	store, err := authstore.DefaultStore()
	if err != nil {
		return fmt.Errorf("init credential store: %w", err)
	}

	client, err := api.NewClient(cfg.APIURL, nil, store)
	if err != nil {
		return fmt.Errorf("build api client: %w", err)
	}

	state, err := randomState()
	if err != nil {
		return fmt.Errorf("generate login state: %w", err)
	}

	flowCtx, cancel := context.WithTimeout(ctx, browserLoginTimeout)
	defer cancel()

	callbackURL, resultCh, shutdown, err := startCallbackServer(flowCtx, state)
	if err != nil {
		return fmt.Errorf("start local callback server: %w", err)
	}
	defer shutdown()

	loginURL, err := buildLoginURL(cfg.FrontendURL, callbackURL, state)
	if err != nil {
		return fmt.Errorf("build login url: %w", err)
	}

	opened := openBrowser(loginURL)
	if opened {
		_, _ = fmt.Fprintln(w, "Opening Papermap in your browser to sign in...")
	} else {
		_, _ = fmt.Fprintln(w, "Could not open a browser automatically.")
	}
	_, _ = fmt.Fprintln(w, "If your browser does not open, paste this URL:")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, loginURL)
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Waiting for browser login...")

	var code string
	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		code = res.code
	case <-flowCtx.Done():
		if errors.Is(flowCtx.Err(), context.DeadlineExceeded) {
			return errors.New("timed out waiting for browser login")
		}
		return flowCtx.Err()
	}

	tokens, err := client.ExchangeCLICode(ctx, code, state)
	if err != nil {
		return fmt.Errorf("exchange cli code: %w", err)
	}

	cred, err := tokens.ToCredentials(authstore.Credentials{})
	if err != nil {
		return fmt.Errorf("decode credentials: %w", err)
	}

	if err := store.Save(cred); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	// Best-effort warm-up. Failure here should not block login success.
	if _, werr := client.UnifiedWorkspace(ctx); werr != nil {
		_ = werr
	}

	display := strings.TrimSpace(cred.User.Email)
	if display == "" {
		display = "your Papermap account"
	}
	_, _ = fmt.Fprintf(w, "Signed in as %s.\n", display)
	return nil
}

// randomState returns 32 cryptographically random bytes encoded as
// base64url with no padding. The result is safe to drop into a query
// string and to compare with constant-time equality.
func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// buildLoginURL composes the frontend login URL with the cli_callback
// and state query parameters preserved through the existing login flow.
// The frontend reads these and, after the user authenticates, mints a
// one-time CLI code and redirects to callbackURL.
func buildLoginURL(frontendURL, callbackURL, state string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if base == "" {
		return "", errors.New("frontend url is empty")
	}
	u, err := url.Parse(base + "/auth/login")
	if err != nil {
		return "", fmt.Errorf("parse frontend url: %w", err)
	}
	q := u.Query()
	q.Set("cli_callback", callbackURL)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// startCallbackServer binds a loopback listener on a kernel-assigned
// port, registers a single /callback handler, and returns the callback
// URL the frontend should redirect to. The result channel receives
// exactly one callbackResult; subsequent /callback hits get a polite
// "already complete" page. The caller MUST invoke shutdown when done.
func startCallbackServer(ctx context.Context, expectedState string) (
	callbackURL string,
	result <-chan callbackResult,
	shutdown func(),
	err error,
) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, fmt.Errorf("listen on loopback: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	callbackURL = fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)

	ch := make(chan callbackResult, 1)
	var delivered atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		setSecurityHeaders(w)

		if delivered.Load() {
			writeCallbackPage(w, callbackPageData{
				Title:    "Sign-in already complete",
				Subtitle: "You can close this page and return to your terminal.",
				IsError:  false,
			})
			return
		}

		query := r.URL.Query()
		code := query.Get("code")
		state := query.Get("state")

		switch {
		case code == "" || state == "":
			writeCallbackPage(w, callbackPageData{
				Title:    "Sign-in failed",
				Subtitle: "Missing authorization code. Return to your terminal and try again.",
				IsError:  true,
			})
			if delivered.CompareAndSwap(false, true) {
				ch <- callbackResult{err: errors.New("callback missing code or state")}
			}
		case subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1:
			writeCallbackPage(w, callbackPageData{
				Title:    "Sign-in failed",
				Subtitle: "State mismatch. Return to your terminal and try again.",
				IsError:  true,
			})
			if delivered.CompareAndSwap(false, true) {
				ch <- callbackResult{err: errors.New("callback state mismatch")}
			}
		default:
			writeCallbackPage(w, callbackPageData{
				Title:    "Signed in to Papermap",
				Subtitle: "You can close this page and return to your terminal.",
				IsError:  false,
			})
			if delivered.CompareAndSwap(false, true) {
				ch <- callbackResult{code: code}
			}
		}
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() { _ = srv.Serve(ln) }()

	// Tear the server down if the caller's context expires before they
	// call shutdown. Belt-and-suspenders for the timeout path.
	go func() {
		<-ctx.Done()
		sctx, scancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
	}()

	shutdown = func() {
		sctx, scancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
	}
	return callbackURL, ch, shutdown, nil
}

// setSecurityHeaders applies headers we want on every callback response.
// These pages contain no third-party content and no token data, but we
// still want to keep them out of caches and prevent referer leakage of
// the (already used) authorization code.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

// callbackPageData is the template input for the success/error pages.
type callbackPageData struct {
	Title    string
	Subtitle string
	IsError  bool
}

// callbackPageTmpl renders the centered Papermap success/error page.
// No image assets — the wordmark is rendered as text. Background and
// text colors adapt via prefers-color-scheme. A 3-line replaceState
// script scrubs the code and state out of the address bar after render.
var callbackPageTmpl = template.Must(template.New("callback").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light dark; }
    html, body { margin: 0; padding: 0; min-height: 100vh; background: #ffffff; }
    body {
      display: flex; flex-direction: column; align-items: center;
      padding: 22vh 24px 24px;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      color: #0a0a0a;
      text-align: center;
    }
    .wordmark {
      font-size: 32px; font-weight: 700; letter-spacing: -0.02em;
      margin: 0 0 36px;
    }
    h1 { font-size: 26px; font-weight: 600; margin: 0 0 10px; letter-spacing: -0.01em; }
    p  { font-size: 15px; color: #6b7280; margin: 0; max-width: 28rem; line-height: 1.5; }
    .error h1 { color: #b91c1c; }

    @media (prefers-color-scheme: dark) {
      html, body { background: #0a0a0a; color: #f5f5f5; }
      p { color: #9ca3af; }
      .error h1 { color: #f87171; }
    }
  </style>
</head>
<body class="{{if .IsError}}error{{end}}">
  <div class="wordmark">Papermap</div>
  <h1>{{.Title}}</h1>
  <p>{{.Subtitle}}</p>
  <script>
    if (window.history && window.history.replaceState) {
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  </script>
</body>
</html>
`))

// writeCallbackPage renders a callback HTML response. Errors during
// template execution are intentionally swallowed; the user has already
// completed (or failed) the auth flow, and we don't want to leak
// internals via an HTTP 500.
func writeCallbackPage(w http.ResponseWriter, data callbackPageData) {
	_ = callbackPageTmpl.Execute(w, data)
}

// openBrowser tries to launch the user's default browser pointed at
// rawURL. Returns true on success. A false return is non-fatal: the
// caller will print the URL and ask the user to open it manually.
func openBrowser(rawURL string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	// We don't wait for the browser process; on macOS `open` exits
	// immediately, but xdg-open may stay attached.
	go func() { _ = cmd.Wait() }()
	return true
}
