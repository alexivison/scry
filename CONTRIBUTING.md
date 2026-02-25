# Contributing to Scry

Thank you for your interest in contributing to Scry.

## Getting started

### Prerequisites

- Go 1.22 or later
- Git

### Build and test

```bash
git clone https://github.com/alexivison/scry.git
cd scry
go build ./cmd/scry
go test ./...
```

### Run with race detection

```bash
go test -race ./...
```

## Development guidelines

### Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Prefer early guard returns over nested conditionals.
- Keep comments short and only where logic is non-obvious.

### Architecture boundaries

Scry has strict module boundaries. Please respect them:

- **`model`**, **`source`**, **`diff`**, **`search`**, **`review`** — UI-agnostic core logic. No imports from `ui`.
- **`ui`** — Rendering and key handling only. No direct git command execution.
- **`gitexec`** — The only package that runs subprocess commands.
- **`app`** — Wiring only. No business logic.

See [SPEC.md](SPEC.md) for the full architecture reference.

### Testing

- Every feature needs tests. See the task breakdown in SPEC.md for expected test patterns.
- Use `testdata/repos/` fixture repositories for integration tests.
- Golden tests go in `testdata/golden/`.

### Scope discipline

Scry is deliberately minimal. Before proposing a feature, check the non-goals in [SPEC.md](SPEC.md). If your idea adds write operations, plugin infrastructure, or broad Git client functionality, it is likely out of scope for the foreseeable future.

## Submitting changes

1. Fork the repository and create a branch from `main`.
2. Make your changes with tests.
3. Ensure `go test ./...` and `go vet ./...` pass.
4. Open a pull request with a clear description of what changed and why.

## Reporting issues

Use [GitHub Issues](https://github.com/alexivison/scry/issues). Include:

- What you expected to happen
- What actually happened
- Steps to reproduce
- Terminal and OS information (if relevant)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
