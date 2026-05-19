# Contributing to Helix Gateway

Thank you for your interest in contributing! This guide will help you get started.

## How to Contribute

### Reporting Bugs

Open an issue and include:
- Helix version (`helix --version`)
- Go version and OS
- Minimal reproduction steps
- Expected vs actual behavior
- Relevant logs

### Suggesting Features

Open an issue with the `enhancement` label. Describe the use case, not just the solution — understanding the problem helps us find the best approach.

### Submitting Pull Requests

1. Fork the repository and create a branch from `main`.
2. Run `go build ./...` and `go test ./...` — both must pass.
3. Keep changes focused: one feature or fix per PR.
4. Update documentation if behavior changes.
5. Add or update tests for new functionality.

```bash
# Development workflow
go build ./...
go test ./...
go vet ./...

# Run a local instance
go run ./cmd/main.go
```

## Code Style

- Standard Go formatting (`gofmt`).
- No inline comments unless the *why* is non-obvious.
- Error values use sentinel types, not raw strings.
- Prefer table-driven tests.

## Plugin Development

External plugins use the SDK at `sdk/sdk.go`. See the [Plugin Guide](README.md#writing-external-plugins) in the README for a complete example.

## Commit Messages

Use conventional commits:

```
feat: add weighted round-robin load balancing
fix: prevent race in route table hot-reload
docs: add k8s operator CRD reference
```

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
