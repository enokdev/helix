# Lifecycle

Helix manages the application lifecycle — startup, shutdown, and signal handling — so you can hook in at the right moments without writing signal handlers manually.

## The Lifecycle Interface

```go
// core/lifecycle.go
type Lifecycle interface {
    OnStart() error
    OnStop(ctx context.Context) error
}
```

Any component registered in the DI container that implements this interface participates in the lifecycle automatically.

## Startup

`OnStart` is called after all dependencies are resolved, in **topological dependency order**. A component's `OnStart` runs only after all its dependencies have started successfully.

```go
type DatabaseConnection struct {
    helix.Component
    db *sql.DB
}

func (d *DatabaseConnection) OnStart() error {
    // Verify connectivity before the HTTP server accepts requests
    if err := d.db.Ping(); err != nil {
        return fmt.Errorf("database unreachable: %w", err)
    }
    slog.Info("database connected")
    return nil
}
```

If any `OnStart` returns an error, the application exits before the HTTP server starts.

## Shutdown

`OnStop` is called in **reverse startup order** — the last component to start is the first to stop. All `OnStop` calls share a deadline context derived from `WithShutdownTimeout`.

```go
func (d *DatabaseConnection) OnStop(ctx context.Context) error {
    slog.Info("closing database connection")
    return d.db.Close()
}
```

The HTTP server lifecycle (managed by the web starter) drains in-flight requests before database and other components stop.

## Signal Handling

When you use `helix.Run`, Helix installs a signal handler for `SIGTERM` and `SIGINT` (Ctrl+C). On signal:

1. HTTP server stops accepting new requests
2. In-flight requests are given time to complete
3. `OnStop` is called on all lifecycle components in reverse order
4. Application exits cleanly

## Shutdown Timeout

Configure the maximum time allowed for shutdown:

```yaml
# config/application.yaml
helix:
  shutdown-timeout: 30s
```

Or in code:

```go
helix.Run(helix.App{
    ShutdownTimeout: 45 * time.Second,
    Components:      []any{...},
})
```

The default is **30 seconds**. If any component's `OnStop` blocks beyond the budget, `ErrShutdownTimeout` is returned and the process exits.

## Common Patterns

### Background worker

```go
type ReportWorker struct {
    helix.Component
    ticker *time.Ticker
    done   chan struct{}
}

func (w *ReportWorker) OnStart() error {
    w.ticker = time.NewTicker(1 * time.Hour)
    w.done = make(chan struct{})
    go func() {
        for {
            select {
            case <-w.ticker.C:
                w.generateReport()
            case <-w.done:
                return
            }
        }
    }()
    return nil
}

func (w *ReportWorker) OnStop(ctx context.Context) error {
    w.ticker.Stop()
    close(w.done)
    return nil
}
```

### Database connection pool

```go
type PostgresDB struct {
    helix.Component
    DB  *sql.DB
    url string
}

func NewPostgresDB(url string) *PostgresDB {
    return &PostgresDB{url: url}
}

func (p *PostgresDB) OnStart() error {
    db, err := sql.Open("postgres", p.url)
    if err != nil {
        return err
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    if err := db.PingContext(context.Background()); err != nil {
        return err
    }
    p.DB = db
    return nil
}

func (p *PostgresDB) OnStop(ctx context.Context) error {
    return p.DB.Close()
}
```

### External connection (Kafka, Redis, etc.)

```go
type KafkaProducer struct {
    helix.Component
    producer sarama.SyncProducer
    cfg      config.Loader `inject:"true"`
}

func (k *KafkaProducer) OnStart() error {
    brokers, _ := k.cfg.Lookup("kafka.brokers")
    // initialize producer...
    return nil
}

func (k *KafkaProducer) OnStop(ctx context.Context) error {
    return k.producer.Close()
}
```

## Startup and shutdown order

Components start in topological dependency order and stop in reverse. Given:

```
UserController → UserService → UserRepository → DatabaseConnection
```

Startup sequence:

```
1. DatabaseConnection.OnStart()   ← no dependencies
2. UserRepository.OnStart()       ← depends on DatabaseConnection
3. UserService.OnStart()          ← depends on UserRepository
4. UserController.OnStart()       ← depends on UserService
5. HTTP server starts accepting requests
```

Shutdown sequence (reverse):

```
1. HTTP server stops accepting new requests, drains in-flight
2. UserController.OnStop()
3. UserService.OnStop()
4. UserRepository.OnStop()
5. DatabaseConnection.OnStop()    ← last to stop
```

This guarantees the database is still available while the HTTP server processes its last requests.

## What happens if `OnStart` panics?

Panics inside `OnStart` are caught and treated as startup failures. The application exits with a log entry:

```json
{"level":"ERROR","msg":"component startup panicked","component":"*DatabaseConnection","recover":"runtime error: invalid memory address"}
```

Other components that have already started successfully will have their `OnStop` called (cleanup is attempted).

## What happens if `OnStart` returns an error?

The application exits immediately:

```json
{"level":"ERROR","msg":"component failed to start","component":"*DatabaseConnection","error":"connection refused"}
```

Components that started before the failing one have their `OnStop` called.

## Partial shutdown

If `OnStop` returns an error, it is logged but shutdown continues — Helix always calls `OnStop` on all started components regardless:

```json
{"level":"WARN","msg":"component failed to stop cleanly","component":"*CacheClient","error":"connection reset by peer"}
```

This prevents a single stuck component from blocking the entire shutdown.

## Shutdown timeout details

The shutdown budget is shared across all `OnStop` calls. If the total time exceeds `shutdown-timeout`:

```json
{"level":"ERROR","msg":"shutdown timed out","timeout":"30s","components_not_stopped":["*ReportWorker"]}
```

The process then exits anyway. Set the timeout high enough for your slowest `OnStop` call (usually the HTTP request drain):

```yaml
helix:
  shutdown-timeout: 45s  # set to max(HTTP drain time) + 10s buffer
```

## Direct Container Usage

If you manage the container yourself (outside `helix.Run`):

```go
container := core.NewContainer()
container.Register(&DatabaseConnection{db: db})
container.Register(&MyService{})

// Start all lifecycle components in order
if err := container.Start(); err != nil {
    log.Fatal(err)
}

// Shutdown (typically deferred or triggered by signal)
defer container.Shutdown()
```
