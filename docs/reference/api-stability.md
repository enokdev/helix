# API Stability

Helix is pre-v1. The project follows semantic versioning, but compatibility guarantees are intentionally narrower before `v1.0.0`.

## Stable By Default

The following packages are treated as public API. Patch releases should not break source compatibility for documented exported identifiers in these packages:

- `core`
- `web`
- `data`
- `config`
- `starter`
- `security`
- `scheduler`
- `cli`
- `testutil`

## Experimental Surface

The following surfaces may change in minor releases before v1:

- Generated code shape from `helix generate`.
- Code generation directives that have not yet been documented in reference pages.
- `internal` packages.
- Example-only helpers.
- Behavior explicitly called experimental in package documentation.

## Deprecation

When an exported public API needs to change, Helix will prefer this sequence:

1. Add the replacement API.
2. Mark the old API as deprecated in Go doc.
3. Keep both APIs for at least one minor release when practical.
4. Remove the deprecated API only in a minor pre-v1 release or a major release.

## Breaking Changes Before v1

Before `v1.0.0`, breaking changes can still happen in minor releases. Release notes must identify:

- The affected package.
- The old behavior.
- The replacement path.
- Whether automated migration is available.

Patch releases should be reserved for bug fixes, security fixes, documentation, and compatibility-preserving improvements.
