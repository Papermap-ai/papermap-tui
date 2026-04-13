## Papermap TUI

Papermap terminal application.
Built using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). Uses the [Elm Architecture](https://guide.elm-lang.org/architecture/) for TUI rendering.

### Commands

```bash
make run
make test
make fmt
```

### Testing

All project tests live under `test/`.

Run the full test suite:

```bash
go test ./...
```

Or use the Make target:

```bash
make test
```

Run a specific test package:

```bash
go test ./test/auth
go test ./test/api
```
