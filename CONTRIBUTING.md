# Contributing to Helix

Thank you for your interest in contributing to Helix!

## Development Setup

```bash
git clone https://github.com/enokdev/helix.git
cd helix
go mod tidy
```

## Coding Standards

- Follow idiomatic Go conventions (no magic, no global state)
- Package names: lowercase, singular, no underscores (`core`, `config`, `web`)
- All public APIs must have godoc comments
- Run `golangci-lint run` before submitting

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(core): implement DI container with ReflectResolver
fix(web): correct convention routing for DELETE methods
test(data): add integration tests for GormRepository
```

Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`

## Pull Requests

1. Fork the repository and create a branch from `develop`
2. Ensure all tests pass: `go test ./...`
3. Ensure linting passes: `golangci-lint run`
4. Open a PR targeting `develop`

## License

By contributing, you agree your contributions will be licensed under the MIT License.
