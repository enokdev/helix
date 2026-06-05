# Production Readiness

This page documents current production limits by package and the recommended mitigations. It is intentionally conservative: if a behavior is not implemented or continuously verified, treat it as a limit.

## `core`

- Reflection DI is the default and is suitable for most services, but compile-time wiring is preferred for latency-sensitive startup paths. Use `helix generate wire` when startup variance matters.
- Lifecycle shutdown depends on components respecting `OnStop(ctx context.Context)`. Components that ignore cancellation can consume the shutdown budget.
- The container detects duplicate registrations and dependency cycles, but it does not replace architectural package boundaries.

## `config`

- Configuration precedence is `ENV > profile YAML > application.yaml > default`. Avoid relying on implicit defaults for production secrets.
- Config reload hooks are process-local. Distributed config propagation is application-owned.
- Keep secrets in environment variables or the platform secret store, not in YAML files.

## `web`

- The HTTP implementation is Fiber-backed internally; public application code should use `web.Context` and public Helix interfaces only.
- Cache interceptor storage is in-memory and process-local. It supports TTL, max entries, sweep, and cold-key coalescing, but it is not a distributed cache.
- Protect admin-style routes such as `/actuator/metrics` with a guard when they are exposed outside a private network.
- Handler deadlines and cancellation are available through `web.Context.Context()`, but handlers must check the context for long-running work.

## `data` and `data/gorm`

- SQLite migrations are supported by the CLI today. PostgreSQL/MySQL migration execution is not enabled yet, although repository integration tests cover dialect behavior when DSNs are supplied.
- SQLite migrations require CGo because they use `github.com/mattn/go-sqlite3`.
- Configure database pool limits explicitly for production workloads.
- The generic repository is a convenience layer; use custom queries for hot paths or complex SQL.

## `observability`

- `/actuator/health`, `/actuator/metrics`, and `/actuator/info` are available, but health coverage depends on registering indicators for every external dependency.
- OTLP tracing supports endpoint, headers, insecure mode, TLS, and mTLS options. Use `insecure: false` with TLS settings for production collectors.
- Metrics cardinality remains the application owner's responsibility.

## `security`

- JWT validation requires a strong secret or trusted key source. Do not use sample secrets in production.
- RBAC guards protect routes only when registered globally or on the relevant route/controller.
- Token revocation and session management are application-level concerns.

## `scheduler`

- Cron scheduling is process-local. In multi-instance deployments, the same job can run on every replica.
- Distributed locks are not implemented yet. Use a single scheduler replica or an application-owned lock for jobs with side effects.
- Long-running jobs should respect context cancellation during shutdown.

## `starter`

- Starters auto-configure common infrastructure and roll back framework-owned registrations when a later registration step fails.
- Auto-detection depends on module contents and explicit config. In production, prefer explicit starter config for critical infrastructure.

## `cli`

- `helix run` is a development command with hot reload. Use `helix build` and run the built binary in production.
- Named commands accept flags before or after the positional name, for example `helix generate module --dir ./app user` and `helix generate module user --dir ./app`.
- Migrations compile in an isolated temporary module. Keep migration imports self-contained.

## `testutil`

- Test helpers are intended for application and framework tests, not production wiring.
- Mocks replace registered components in a test container only; they do not affect global auto-registration outside that test process.

## Release Checks

Before publishing a production-facing release, run:

```bash
go test ./...
go test ./core ./observability ./web ./data/gorm -run '^$' \
  -bench 'Benchmark(ReflectResolver|ActuatorHealthRoute|BindingJSON|CacheInterceptorHit|RepositoryFindByIDSQLite)' \
  -benchmem
govulncheck ./...
```

See also [Performance Reference](./performance.md), [Deployment](./deployment.md), and [API Stability](./api-stability.md).
