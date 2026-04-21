# Papermap TUI Agent Guide

## Project Overview

Papermap TUI is a terminal-native Go application that provides a focused
Papermap workflow:

`install -> launch -> authenticate -> land in unified workspace -> ask
insights -> see streamed results`

The MVP should stay narrow. Prefer the smallest end-to-end flow that works
well over broader feature coverage.

## Current Status

This repository is in early implementation. The initial plan lives in
`Plans/initial_plan.md`. 

When making architecture decisions, align with the MVP scope from the plan:

- Email/password login directly in the TUI.
- Unified workspace as the default landing experience.
- Streaming insight responses in the terminal.
- Configurable API base URL via config file and environment variable.
- Lightweight distribution path with `go install` first.

Do not quietly expand scope into dashboard editing, billing, multi-user
collaboration, or broad workspace/data-source management unless explicitly
requested.

## Module And Stack

- Module path: `github.com/papermap/papermap-tui`.
- Language: Go 1.26+.
- UI framework: `charm.land/bubbletea/v2`.
- Styling: `charm.land/lipgloss/v2`.
- Markdown rendering: `charm.land/glamour/v2`.
- Additional UI components: Bubbles when needed.
- HTTP and streaming: standard library `net/http` and SSE handling.

## Architecture Direction

Expected structure:

```text
cmd/papermap/main.go
internal/app/
internal/api/
internal/auth/
internal/config/
internal/theme/
internal/ui/
```

Preferred responsibilities:

- `cmd/papermap`: CLI entry point.
- `internal/app`: root Bubble Tea model, app state, routing, key handling.
- `internal/api`: backend client and Papermap endpoint integrations.
- `internal/auth`: login, token refresh, credential persistence.
- `internal/config`: config loading and environment override handling.
- `internal/theme`: shared lipgloss styles and layout constants.
- `internal/ui`: screen-level UI models and reusable UI components.

Bubble Tea should remain the top-level orchestration model. Screen transitions
should happen through messages and explicit state, not hidden globals.

## Product And UX Expectations

The experience should feel keyboard-first, branded, minimal, and polished.

Baseline screens:

- Landing.
- Login.
- Main chat / insight screen.
- Workspace picker.

Default MVP assumptions:

- Landing should guide the user into authentication or directly into the main
  workspace if already authenticated.
- The unified workspace should be the default workspace after login.
- Insight responses should stream when possible.
- Output should remain terminal-friendly: markdown, summaries, and readable
  tables.

Any work inside `internal/ui` must also follow `internal/ui/AGENTS.md`.

## Backend Integration

Prefer existing Papermap backend routes over inventing new flows.

The MVP plan assumes these existing backend capabilities:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `GET /api/v1/analytics/workspaces/unified`
- `GET /api/v1/analytics/workspaces/paginate`
- `GET /api/v1/analytics/workspaces/{id}`
- `POST /api/v1/analytics/charts/stream`
- `POST /api/v1/analytics/requests/stream`
- `GET /api/v1/analytics/chats/{id}/conversations`

Before introducing backend changes, verify that the needed behavior is truly
missing.

## Key Engineering Patterns

### Config

Configuration should be loaded from:

- `~/.papermap/config.yaml`
- `PAPERMAP_API_URL`

Environment variables should override file configuration.

Prefer passing config through constructors or app state. Do not introduce
package-level mutable configuration state unless there is a very strong reason.

### Auth And Credentials

Credential storage path:

- `~/.papermap/credentials`

Requirements:

- File permissions must be `0o600`.
- Never log tokens, passwords, or credential payloads.
- Mask password input in the UI.
- Zero password bytes after use when practical.
- Check token expiry before API calls when auth state exists.
- Attempt token refresh before forcing re-authentication.

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

Current build tool choice: Makefile.

Preferred commands once targets exist:

- Build: `go build .` or `make build`
- Run: `go run ./cmd/papermap` or `make run`
- Test: `go test ./...` or `make test`
- Format: `gofumpt -w .` or `make fmt`
- Lint: project-specific lint target if added

Distribution expectations:

- Phase 1: `go install github.com/papermap/papermap-tui/cmd/papermap@latest`
- Phase 2: GitHub Releases and Homebrew tap

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
- Add comments sparingly. Prefer self-explanatory code.

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
