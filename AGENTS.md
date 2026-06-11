# Papermap TUI Agent Guide

## Project Overview

Papermap TUI is a terminal-native Go application that provides a focused
Papermap workflow:

`install -> launch -> authenticate -> land in unified workspace -> ask
insights -> see streamed results`

The MVP should stay narrow. Prefer the smallest end-to-end flow that works
well over broader feature coverage.

When making architecture decisions, align with the MVP scope:

- Authentication via `papermap auth` CLI subcommands (huh-based prompts).
- Unified workspace as the default landing experience.
- Streaming insight responses in the terminal.
- Configurable API base URL via config file, env var, or `--api-url` flag.
- Lightweight distribution: `go install`, `install.sh`, GoReleaser.

Do not quietly expand scope into dashboard editing, billing, multi-user
collaboration, or broad workspace/data-source management unless explicitly
requested.

## Module And Stack

- Module path: `github.com/papermap/papermap-tui`.
- Language: Go (see `go.mod` for the pinned version).
- UI framework: `charm.land/bubbletea/v2`.
- Forms / prompts: `charm.land/huh/v2` (used by the `auth` CLI).
- Styling: `charm.land/lipgloss/v2`.
- Markdown rendering: `charm.land/glamour/v2`.
- Additional UI components: Bubbles when needed.
- HTTP and streaming: standard library `net/http` and SSE handling.
- Credential storage: `github.com/99designs/keyring` with file fallback.

## Architecture Direction

Expected structure:

```text
cmd/papermap/main.go
internal/app/
internal/api/
internal/auth/
internal/cli/auth/
internal/config/
internal/teatest/
internal/theme/
internal/ui/
```

Preferred responsibilities:

- `cmd/papermap`: CLI entry point and subcommand dispatch.
- `internal/app`: root Bubble Tea model, app state, routing, key handling.
- `internal/api`: backend client and Papermap endpoint integrations.
- `internal/auth`: credential persistence (keyring + file fallback) and
  token expiry parsing.
- `internal/cli/auth`: `papermap auth login | logout | whoami` flows.
- `internal/config`: config loading and environment override handling.
- `internal/teatest`: shared Bubble Tea test helpers.
- `internal/theme`: shared lipgloss styles and layout constants.
- `internal/ui`: screen-level UI models and reusable UI components.

Bubble Tea should remain the top-level orchestration model. Screen transitions
should happen through messages and explicit state, not hidden globals.

## Product And UX Expectations

The experience should feel keyboard-first, branded, minimal, and polished.

Baseline screens:

- Landing (also used as the signed-out exit screen).
- Main chat / insight screen (default landing when authenticated).
- Workspace picker.

Default MVP assumptions:

- Authentication happens out-of-band via `papermap auth login` before the
  TUI starts. The TUI itself does not collect credentials.
- The unified workspace is the default workspace after launch.
- Insight responses should stream when possible.
- Output should remain terminal-friendly: markdown, summaries, and readable
  tables.

Any work inside `internal/ui` must also follow `internal/ui/AGENTS.md`.

## Backend Integration

Prefer existing Papermap backend routes over inventing new flows. The
backend repo (and its OpenAPI spec) is the source of truth for available
endpoints.

Before introducing backend changes, verify that the needed behavior is truly
missing.

## Key Engineering Patterns

### Config

Configuration is loaded from, in precedence order:

1. `--api-url` flag (root or `auth login`).
2. `~/.papermap/config.yaml`.
3. Built-in default (see `internal/config`).

Cached workspace metadata lives in `~/.papermap/workspaces.json` and is
cleared on logout.

Prefer passing config through constructors or app state. Do not introduce
package-level mutable configuration state unless there is a very strong reason.

### Auth And Credentials

Credentials are stored in the OS keyring (Keychain, Secret Service,
WinCred, KWallet) via `github.com/99designs/keyring`. When no supported
backend is available, or when `PAPERMAP_FORCE_FILE_STORE=1` is set, the
file fallback is used at `~/.papermap/credentials`.

Requirements:

- File fallback permissions must be `0o600`.
- Never log tokens, passwords, or credential payloads.
- Mask password input in CLI prompts (huh `EchoModePassword`).
- Zero password bytes after use when practical.
- Check token expiry before API calls when auth state exists.
- Attempt token refresh before forcing re-authentication.
- On logout, attempt remote revoke, then clear local credentials and the
  workspace cache.

### API Client

Keep backend access centralized in `internal/api`.

Expected client responsibilities:

- Base URL handling.
- Auth header injection.
- Request/response decoding.
- Refresh-aware auth retry behavior where appropriate.
- SSE streaming support for insight responses.

### State Management

Prefer explicit state over implicit coupling. A likely root model shape is:

```text
RootModel
- current screen
- auth state
- workspace state
- chat state
- config
```

Keep screen-specific concerns inside screen models, but avoid fragmenting the
app into too many tiny abstractions too early.

## Build, Run, And Release

The project uses a `Makefile` plus GoReleaser for releases.

Common commands:

- Build: `make build` (writes `bin/papermap`)
- Run: `make run`
- Test: `make test` (race + count=1)
- Format: `make fmt` (gofumpt + goimports)
- Lint: `make lint` (golangci-lint)
- Install git hooks: `make hooks`
- Snapshot release: `make release-snapshot`
- Clean: `make clean`

The pre-commit hook runs `make lint`, so local commits will fail if lint
fails. Linting is enforced locally only; CI does not run it. CI runs test
and build on macOS on every push and PR to `main`. Ubuntu is temporarily
disabled while all current testers are on macOS.

Distribution paths:

- `go install github.com/papermap/papermap-tui/cmd/papermap@latest`
- `curl -sSL https://raw.githubusercontent.com/Papermap-ai/papermap-tui/main/install.sh | sh`
- GitHub Releases via GoReleaser (darwin/linux × amd64/arm64).
- Homebrew tap (planned).

Prefer implementation choices that keep cross-platform builds simple.

## Go Style Guidelines

- Use `goimports`-compatible import grouping: stdlib, external, internal.
- Use `gofumpt` formatting.
- Follow standard Go naming conventions.
- Pass `context.Context` as the first parameter for operations that can block,
  do I/O, or depend on request lifetime.
- Return errors explicitly. Wrap with `fmt.Errorf` when adding context.
- Keep interfaces small and define them in the consuming package.
- Prefer clear concrete types over premature abstraction.
- Use snake_case JSON tags.
- Use octal notation for file permissions such as `0o600`.
- Log messages should start with a capital letter.

## Comments

- Comments on their own lines should start with a capital letter and end with a
  period.
- Wrap comments near 78 columns when practical.
- Add comments sparingly. Prefer self-explanatory code and function and variable names that are self-documenting.

## Testing Guidelines

- Use table-driven tests when they improve clarity.
- Use `t.Parallel()` when tests are safe to parallelize.
- Use `t.Setenv()` for environment manipulation.
- Use `t.TempDir()` for temporary filesystem state.
- Prefer deterministic tests for UI rendering and formatting behavior.
- For TUI snapshot or golden testing, adopt `charm.land/catwalk` if snapshot
  coverage is introduced.

When tests involve auth or backend interactions, prefer mocking at the client
or transport boundary rather than making live external calls.

## Security Guidelines

- Never commit secrets, tokens, or real credential fixtures.
- Never print passwords or bearer tokens in logs, errors, or test snapshots.
- Be careful with panic paths and formatted errors that may include request
  payloads.
- Validate credential file permissions when reading persisted auth state.

## Commits

- Use semantic commit prefixes: `feat:`, `fix:`, `chore:`, `refactor:`,
  `docs:`, `test:`, `sec:`.
- Keep commit messages one line unless extra context is genuinely necessary.

## Agent Workflow

When working in this repo:

- Read the relevant package before editing.
- Prefer minimal, direct changes over speculative abstractions.
- Keep MVP scope tight.
- Verify behavior with tests or targeted manual checks when possible.
- Format Go files after editing.
- If working on UI code, read `internal/ui/AGENTS.md` first.
