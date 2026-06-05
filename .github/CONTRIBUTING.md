# Contributing to Helix

Thank you for your interest in improving Helix.

## Code of Conduct

By participating in this project, you agree to follow the expectations described in [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).

## Development Environment

- Install Go **1.21 or newer**.
- Fork `enokdev/helix` to your own GitHub account.
- Clone your fork locally.
- Synchronize dependencies with:

```bash
/Users/yacoubakone/.govm/go/bin/go mod tidy
```

If the `go` binary in your `PATH` causes a version mismatch on macOS, use the govm-managed Go toolchain shown above.

## Development Workflow

1. Create a branch from `main`.
2. Make focused, well-scoped changes.
3. Add or update tests when behavior changes.
4. Run the required checks before opening a pull request:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
golangci-lint run
```

5. Push your branch to your fork.
6. Open a pull request against `main`.

## Commit Messages

Use conventional commits:

```text
<type>(<scope>): <description>
```

Examples:

- `feat(core): add lifecycle shutdown hooks`
- `fix(web): preserve route parameter names`
- `docs(config): clarify environment precedence`

Do **not** add `Co-authored-by` trailers.

## Pull Request Process

When you open a pull request:

- Link the related issue when one exists.
- Describe what changed and why.
- Call out any breaking changes, migration steps, or follow-up work.
- Ensure CI passes before requesting review.

Small, focused pull requests are reviewed faster than broad refactors.

## What Makes a Good Contribution

Good contributions are:

- Aligned with the framework goals and package boundaries
- Reproducible, well-described, and easy to validate
- Backed by tests or documentation updates when appropriate
- Respectful of existing conventions, performance expectations, and maintainability

## Issue Triage Labels

The repository may use the following labels to organize work:

- `bug` — confirmed defects or regressions
- `enhancement` — feature requests and improvements
- `documentation` — docs-only work
- `good first issue` — suitable for new contributors
- `help wanted` — needs community contribution
