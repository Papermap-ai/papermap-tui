# Papermap TUI

[![build](https://github.com/Papermap-ai/papermap-tui/actions/workflows/build.yml/badge.svg)](https://github.com/Papermap-ai/papermap-tui/actions/workflows/build.yml)
[![release](https://github.com/Papermap-ai/papermap-tui/actions/workflows/release.yml/badge.svg)](https://github.com/Papermap-ai/papermap-tui/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/papermap/papermap-tui.svg)](https://pkg.go.dev/github.com/papermap/papermap-tui)
[![Go Report Card](https://goreportcard.com/badge/github.com/papermap/papermap-tui)](https://goreportcard.com/report/github.com/papermap/papermap-tui)

Terminal-native access to Papermap. Sign in, land in your unified workspace, ask questions, watch streamed insight responses render in your terminal.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), [Glamour](https://github.com/charmbracelet/glamour), and [Huh](https://github.com/charmbracelet/huh).

![Papermap TUI start page](images/start_page.png)

## Install

### Homebrew (macOS / Linux)

```bash
brew install papermap-ai/papermap/papermap
# alias
brew install papermap-ai/papermap/tui
```

Or tap once, then install by short name:

```bash
brew tap papermap-ai/papermap
brew install papermap
```

### Install script (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Papermap-ai/papermap-tui/main/install.sh | sh
```

The script downloads the latest release, verifies its SHA256 checksum, and installs to `/usr/local/bin` or `~/.local/bin`.

Override the install prefix or version:

```bash
PREFIX=$HOME/.local VERSION=v0.0.1 \
    curl -fsSL https://raw.githubusercontent.com/Papermap-ai/papermap-tui/main/install.sh | sh
```

### Go install

```bash
go install github.com/papermap/papermap-tui/cmd/papermap@latest
```

### From source

```bash
git clone https://github.com/Papermap-ai/papermap-tui.git
cd papermap-tui
make build
./bin/papermap
```

## Getting started

Sign in once, then launch the TUI:

```bash
papermap auth login
papermap
```

`auth login` writes credentials to `~/.papermap/credentials` (mode `0600`). Subsequent launches restore your session automatically and refresh tokens as needed.

### Manage workspaces from the CLI

Create and list workspaces without leaving the terminal. Both commands require a prior `papermap auth login`.

```bash
papermap workspace create   # interactive form
papermap workspace list     # tab-aligned table of your workspaces
```

`workspace create` walks you through a huh form that collects:

- Workspace name
- Database type — `Postgres`, `MySQL`, `MongoDB`, or `Supabase`
- Host, port (defaults to `5432`/`3306`/`27017`/`5432` if left blank), database name, username, password

The form posts to `POST /api/v1/analytics/workspaces`. The backend verifies the
database connection asynchronously, so the command returns as soon as the
workspace row is created and prints `Verifying connection in background.` On
success the local workspace cache (`~/.papermap/workspaces.json`) is refreshed
so the TUI sees the new workspace on next launch.

Only standard databases are supported in the CLI today. OAuth integrations
(Stripe, QuickBooks, Google Ads, etc.) and Sheets/CSV uploads still need to
be created from the web app.

## Usage

```text
papermap [flags] [command]
```

### Flags

| Flag                | Description                                                  |
| ------------------- | ------------------------------------------------------------ |
| `-v`, `--version`   | Print version, commit, and build date                        |
| `-h`, `--help`      | Show help                                                    |
| `-u`, `--user`      | Print the signed-in user (alias for `auth whoami`)           |
| `--api-url <url>`   | Override the API base URL for this run (sets `PAPERMAP_API_URL`) |

### Commands

| Command                | Description                                                                  |
| ---------------------- | ---------------------------------------------------------------------------- |
| _(none)_               | Launch the TUI                                                               |
| `auth login`           | Sign in with email + password                                                |
| `auth logout`          | Clear stored credentials and workspace cache                                 |
| `auth whoami`          | Print the currently signed-in user                                           |
| `workspace create`     | Create a database-backed workspace (Postgres, MySQL, MongoDB, Supabase)      |
| `workspace list`       | List workspaces visible to the signed-in user                                |
| `logout`               | Deprecated alias for `auth logout`                                           |

### Keyboard controls

| Key              | Action                                  |
| ---------------- | --------------------------------------- |
| `Enter`          | Submit prompt / confirm                 |
| `Tab`            | Switch focus inside forms               |
| `Ctrl+W`         | Switch workspace                        |
| `Ctrl+L`         | Clear chat                              |
| `Esc`            | Cancel / go back                        |
| `Ctrl+C`         | Quit (with confirmation)                |
| `!` (line start) | Run a one-shot shell command            |

### Shell mode

Type `!` at the start of an empty prompt to switch to shell mode, then
type the command and hit `Enter` to run it. Output is captured into the
transcript; `Esc` cancels an in-flight command. Each invocation is a
fresh one-shot — there is no persistent shell session.

The shell binary is resolved at TUI startup. On macOS and Linux it
honors `$SHELL` (falling back to `/bin/sh`). On Windows it defaults to
PowerShell 7 (`pwsh.exe`); see [Windows shell selection](#windows-shell-selection)
to opt into `cmd.exe` instead.

## Configuration

Configuration is loaded from `~/.papermap/config.yaml`. Environment variables take precedence.

| Setting       | Config key      | Env var                        | Default                      |
| ------------- | --------------- | ------------------------------ | ---------------------------- |
| API URL       | `api_url`       | `PAPERMAP_API_URL`             | `https://prod.dataapi.papermap.ai` |
| Frontend URL  | `frontend_url`  | `PAPERMAP_FRONTEND_URL`        | `https://papermap.ai`        |
| Windows shell | `shell.windows` | —                              | `pwsh`                       |

Example `~/.papermap/config.yaml`:

```yaml
api_url: https://prod.dataapi.papermap.ai
frontend_url: https://papermap.ai
```

For safety, browser login only allows `https://papermap.ai` (and
`https://www.papermap.ai`) by default. Internal/dev frontends require:

```bash
PAPERMAP_ALLOW_UNTRUSTED_FRONTEND=1 papermap auth login --frontend-url https://internal-frontend.example
```

### Windows shell selection

On Windows the config file lives at `C:\Users\<name>\.papermap\config.yaml`
(same `~/.papermap/` layout as macOS and Linux, resolved via
`%USERPROFILE%`).

`shell.windows` controls which shell `!` mode invokes. Two values are
supported:

- `pwsh` (default) — PowerShell 7+ (`pwsh.exe`). Resolved by globbing
  `%ProgramFiles%\PowerShell\<N>\pwsh.exe` and picking the highest
  installed version.
- `cmd` — `cmd.exe` from `%SystemRoot%\System32`.

To opt out of PowerShell, drop this in your config:

```yaml
shell:
  windows: cmd
```

If `shell.windows` is `pwsh` (or unset) and `pwsh.exe` cannot be found,
`papermap` refuses to start with:

```
papermap: shell.windows is "pwsh" but pwsh.exe was not found under %ProgramFiles%\PowerShell\<N>; install PowerShell 7+ or set shell.windows: cmd
```

To recover, either install PowerShell 7+
(<https://github.com/PowerShell/PowerShell/releases>) or set
`shell.windows: cmd` in the config file above.

The shell is resolved once at TUI startup, so restart `papermap` after
editing the config. Windows PowerShell 5.1 (`powershell.exe`) is
intentionally not a supported value — only `pwsh` (7+) and `cmd`.

## Development

```bash
make build              # build into ./bin/papermap
make run                # go run ./cmd/papermap
make test               # go test ./... -race
make fmt                # gofumpt + goimports
make lint               # golangci-lint run
make tidy               # go mod tidy
make hooks              # install repo git hooks (lint on commit)
make release-snapshot   # local goreleaser snapshot build
```

Project layout:

```text
cmd/papermap/             # CLI entry point + subcommand routing
internal/api/             # Backend HTTP client + SSE streaming
internal/auth/            # Token store and credential persistence
internal/cli/auth/        # `auth login|logout|whoami` huh forms
internal/cli/clitheme/    # Shared huh theme used by CLI subcommands
internal/cli/workspace/   # `workspace create|list` huh forms
internal/config/          # Config + env loading
internal/theme/           # Lipgloss palette and shared styles
internal/ui/              # Bubble Tea screen models (landing/chat/workspace)
internal/app/             # Root app model and orchestration
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, workflow, and pull request guidelines.

## Releasing

Releases are produced by [GoReleaser](https://goreleaser.com) on tag pushes:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The `release` workflow builds darwin/linux × amd64/arm64 archives, uploads them to GitHub Releases, and publishes a `checksums.txt` for the install script to verify against.

## License

See [LICENSE.md](LICENSE.md).
