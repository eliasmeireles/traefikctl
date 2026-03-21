# Contributing to traefikctl

## Development Setup

### Prerequisites

- Go 1.22+
- Make
- golangci-lint

### Getting Started

```bash
git clone https://github.com/eliasmeireles/traefikctl.git
cd traefikctl
go mod download
make build
```

## Development Workflow

```bash
make test    # Run tests with coverage
make lint    # Run linters
make fmt     # Format code
make build   # Build binary
```

## Code Style

- All code formatted with `gofmt`
- Imports organized with `goimports`
- Must pass `golangci-lint`

## Tests

- Use `github.com/stretchr/testify` for assertions
- Test names follow: `must_<expected_behavior>`
- All new features must include tests

## Pull Requests

1. Create a feature branch
2. Make changes with tests
3. Ensure `make test && make lint` passes
4. Open PR with clear description

### Commit Messages

Follow conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
