# Release Process

This page is the maintainer checklist for validating and publishing Helix releases.

## Local Verification

Run these commands before creating a tag:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
/Users/yacoubakone/.govm/go/bin/go build ./...
govulncheck ./...
scripts/smoke_external_install.sh
scripts/smoke_30_minute_crud.sh
```

If `govulncheck ./...` reports a vulnerability, classify it before release:

- **Reachable vulnerability:** update or replace the dependency before release.
- **Unreachable vulnerability:** document why the vulnerable symbol is not reachable, then open a follow-up issue for dependency tracking.
- **No fixed version:** document the upstream advisory, affected package, and mitigation in the release notes.

## GoReleaser Dry Run

Validate the release configuration without publishing:

```bash
goreleaser release --snapshot --clean --skip=publish
```

The dry run must complete without changing tracked files.

## Pre-Release Tag

Before the first stable release, publish a pre-release tag:

```bash
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

The `Release` GitHub Actions workflow must create a draft release from the tag. Verify:

- The release workflow succeeds.
- The generated changelog is readable.
- The draft release does not contain unexpected generated files.
- Installation commands still work from an external module.

## Stable Tag

After the pre-release checks pass, create the stable tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Keep the GitHub release as a draft until the smoke checks and installation commands have been verified from the published tag.

## Installation Checks

From a clean directory outside this repository, verify:

```bash
go get github.com/enokdev/helix@v0.1.0
go install github.com/enokdev/helix/cmd/helix@v0.1.0
helix --version
```

Replace `v0.1.0` with the tag being released.
