# Performance Reference

Helix keeps official micro-benchmarks for the framework surfaces that affect startup and request latency. They are intended for local comparison before release and for investigating regressions; they are not a replacement for application-level load tests.

## Running Benchmarks

```bash
go test ./... -run '^$' -bench 'Benchmark' -benchmem
```

For the maintained release-readiness set:

```bash
go test ./core ./observability ./web ./data/gorm -run '^$' \
  -bench 'Benchmark(ReflectResolver|ActuatorHealthRoute|BindingJSON|CacheInterceptorHit|RepositoryFindByIDSQLite)' \
  -benchmem
```

## Official Benchmarks

| Benchmark | Package | Purpose |
|-----------|---------|---------|
| `BenchmarkRun_ZeroParams` | root | Zero-config application bootstrap path |
| `BenchmarkRunMinimalLifecycle` | root | Minimal app lifecycle startup path |
| `BenchmarkReflectResolverRegisterAndResolve` | `core` | DI registration plus first reflective resolution |
| `BenchmarkReflectResolverResolveSingleton` | `core` | Warm singleton DI resolution |
| `BenchmarkActuatorHealthRoute` | `observability` | `/actuator/health` request path |
| `BenchmarkBindingJSON` | `web` | JSON request binding and validation plan execution |
| `BenchmarkCacheInterceptorHit` | `web` | Cache interceptor hot-key response path |
| `BenchmarkRepositoryFindByIDSQLite` | `data/gorm` | Simple repository lookup through GORM and SQLite |

## Interpreting Results

Run benchmarks on the same machine, Go version, and build flags when comparing two commits. Keep the raw command output in release notes only when it is relevant to a performance claim.

For production validation, benchmark Helix inside the target application with realistic middleware, database, network, TLS, logging, tracing, and payload sizes.
