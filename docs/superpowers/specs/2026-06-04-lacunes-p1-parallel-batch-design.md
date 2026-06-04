# Lacunes P1 Parallel Batch Design

## Context

`lacunes.md` keeps the actionable roadmap for Helix. Most P0 release-readiness items are already closed. This batch intentionally targets P1 production robustness items that can move independently without creating broad conflicts across the framework.

The selected P1 tasks are:

- Add TLS options for OTLP exporters.
- Handle partial starter configuration failures.
- Correct known cache interceptor limits.

`lacunes.md` remains the final status ledger. A task is marked complete only after the implementation, tests, and public or maintainer-facing documentation provide enough proof.

## Goal

Close or materially advance the selected P1 tasks with focused changes. The batch should keep runtime behavior backward compatible unless the current behavior is clearly unsafe or undocumented.

## Non-Goals

- Do not change unrelated P1, P2, or P3 tasks.
- Do not redesign the DI container API unless starter rollback cannot be handled locally.
- Do not remove existing internal planning artifacts.
- Do not mark a `lacunes.md` task complete based only on intent or partial work.

## Approach

Use a surface-independent batch:

1. Work on observability tracing configuration for OTLP TLS.
2. Work on starter configuration failure behavior in starter packages.
3. Audit the cache interceptor before editing it, because current code already appears to include single-flight, maximum size, and periodic sweeping.

These surfaces mostly touch different packages: `observability`, `starter`, and `web`. Shared files such as docs and `lacunes.md` are updated after implementation evidence is collected.

## OTLP TLS Design

Extend `observability.TracingConfig` with explicit OTLP transport options:

- `Insecure bool` controls whether OTLP HTTP uses plaintext/insecure transport.
- `Headers map[string]string` passes static exporter headers.
- TLS-related configuration supports production collectors without forcing insecure mode.

Backward compatibility:

- Existing configs that only set `enabled`, `exporter`, `endpoint`, and `service-name` keep working.
- The current insecure behavior remains available through explicit config.
- `stdout` exporter ignores OTLP-only fields.

Validation:

- Unit tests cover config resolution for insecure mode, headers, and TLS settings.
- Exporter construction tests verify the expected options are accepted without requiring a live collector.
- Docs list the new keys and include a production OTLP example.

## Starter Partial Failure Design

Starter `Configure` methods should avoid leaving invisible partial state when registration fails. The preferred pattern is:

1. Resolve and construct required objects first.
2. Register objects in a deterministic order.
3. If a later registration fails, return an error with clear starter and registration context.
4. Where the container cannot rollback registered components, tests should prove either that no registration occurred before the failure or that the registered component is inert until lifecycle start.

This design avoids adding a broad container rollback API for the first pass. If a specific starter cannot satisfy the contract locally, that starter should receive a narrow helper or a documented diagnostic behavior.

Validation:

- Tests force registration failures in web, data, observability, and scheduling starters where applicable.
- Tests assert that failed configuration does not start external resources or leave active orphan lifecycles.
- Error messages include the starter name and failed component type or role.

## Cache Interceptor Design

Start with an audit, not new code. The current cache implementation appears to already include:

- Cold-key request coalescing.
- Maximum entry count.
- Periodic sweep of expired and oversized state.
- LRU/FIFO eviction behavior.

The task can be closed if tests and docs prove those behaviors satisfy `lacunes.md`. If gaps remain, add only the missing tests or docs first. Code changes are limited to defects found during that audit.

Validation:

- `go test ./web/...` covers concurrent cold-key behavior, TTL eviction, size eviction, and stop behavior.
- Public docs describe cache directive options and production limits.
- `lacunes.md` is marked complete only if the proof is concrete.

## Parallelization

The implementation can be split into independent tracks:

- Observability track: `observability/` and config docs.
- Starter track: `starter/` tests and small starter-local changes.
- Cache track: `web/cache_interceptor.go`, `web/cache_interceptor_test.go`, and docs, but only after audit.

`lacunes.md` is edited last to avoid premature completion status. If multiple workers are used, only one worker should own `lacunes.md` and shared docs consolidation.

## Test Plan

Run targeted tests first:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./observability/...
/Users/yacoubakone/.govm/go/bin/go test ./starter/...
/Users/yacoubakone/.govm/go/bin/go test ./web/...
```

Then run the full suite before marking tasks done:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
```

If docs-only completion is used for the cache task, the targeted web tests still need to pass so the proof remains current.

## Completion Rules

- OTLP TLS is complete when config, tests, and docs cover secure and insecure exporter modes.
- Starter partial failure is complete when each affected starter has targeted failure tests and no active orphan behavior is demonstrated.
- Cache interceptor is complete when existing or added tests prove single-flight, max size, and sweep behavior, and docs describe the supported contract.

Only completed tasks receive `[x]` in `lacunes.md`, with a short `Preuve:` bullet.
