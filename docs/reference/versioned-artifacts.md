# Versioned Artifacts

This page records which tracked files belong in the public repository before release.

## Framework Source

Keep these directories and files:

- `core/`
- `config/`
- `web/`
- `data/`
- `observability/`
- `security/`
- `scheduler/`
- `starter/`
- `cli/`
- `cmd/`
- `testutil/`
- Root Go package files such as `helix.go`, `testapp.go`, `scan.go`, and `errors.go`.

Reason: these are the framework implementation and public package surface.

## Tests and Examples

Keep:

- `*_test.go` files colocated with packages.
- `examples/zero-config/`
- `examples/crud-api/`
- `examples/secured-api/`

Reason: tests validate framework behavior; examples are public onboarding assets.

## Documentation

Keep:

- `README.md`
- `CONTRIBUTING.md`
- `LICENSE`
- `docs/`
- `AGENTS.md`

Reason: these files support users, contributors, maintainers, and local agent workflows.

## CI, Release, and Maintainer Scripts

Keep:

- `.github/workflows/`
- `.goreleaser.yaml` when present.
- `scripts/smoke_external_install.sh`
- `scripts/smoke_30_minute_crud.sh`

Reason: these files provide release, validation, and onboarding checks.

## Internal Agent and Planning Artifacts

Keep when intentionally used by maintainers:

- `.agents/`
- `.claude/`
- `.opencode/`
- `.github/skills/`
- `docs/superpowers/`
- `lacunes.md`

Reason: these artifacts preserve project workflow context. They are not part of the Go framework API.

## Generated or Accidental Files

Before release, inspect tracked files for:

- Local diff dumps such as `*.diff` and `story_diff.txt`.
- Duplicate or malformed module files such as `go 2.sum`.
- Temporary clarification notes that duplicate public docs.

The current tracked inventory includes files matching those checks. They remain tracked until a maintainer decides whether to keep, replace, or remove them:

- `00_START_HERE.md`
- `CLARIFICATIONS_SUMMARY.txt`
- `HTTP_LAYER_CLARIFICATIONS.md`
- `IMPLEMENTATION_GUIDE.md`
- `QUICK_REFERENCE.txt`
- `README_CLARIFICATIONS.md`
- `go 2.sum`
- `story_13_5.diff`
- `story_14_1.diff`
- `story_diff.txt`

If a file is removed, the commit should explain why it is accidental or replace it with public documentation.

## Audit Command

Run this command before a release:

```bash
git ls-files
```

Then compare the output against the categories above. Any tracked file outside these categories needs either a documented reason to stay or a cleanup commit.
