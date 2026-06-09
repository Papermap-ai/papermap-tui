# Contributing to Papermap TUI

Thanks for your interest in improving Papermap TUI. Contributions of all
sizes are welcome: bug reports, fixes, docs, and new features.

## Getting started

1. Fork the repository and clone your fork.
2. Make sure you have Go 1.26+ installed.
3. Build and run locally:

   ```bash
   make run
   ```

4. Run the test suite and linter before pushing:

   ```bash
   make lint
   go test ./...
   ```

## Making changes

- Create a branch off `main` (for example `feat/search-improvements`,
  `fix/config-parse`).
- Keep changes focused: one logical change per pull request.
- Follow existing code style. Run `make fmt` before committing.
- Add or update tests when changing behavior.
- Update the README if you change user-facing behavior or flags.

## Commit messages

Use short, descriptive messages in the imperative mood, for example:

```text
add fuzzy match scoring
fix crash when SSH config is missing
```

## Submitting a pull request

1. Push your branch and open a PR against `main`.
2. Describe what changed and why. Link any related issue.
3. Make sure CI is green.
4. Be open to feedback. Reviews are about the code, not the author.

## Reporting bugs / requesting features

Open an issue using the provided templates. Please include:

- What you expected to happen.
- What actually happened.
- Steps to reproduce.
- Your OS and Papermap version (`papermap --version`).

## Code of Conduct

By participating in this project, you agree to abide by the
[Code of Conduct](./CODE_OF_CONDUCT.md).
