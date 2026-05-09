# Scheduling

Helix includes a cron-based scheduler with graceful shutdown, panic recovery, and concurrency control — activated automatically when `github.com/robfig/cron/v3` is in your `go.mod`.

## Defining Jobs

Implement `scheduler.ScheduledJobProvider` to register cron jobs:

```go
import "github.com/enokdev/helix/scheduler"

type ReportJobs struct {
    helix.Component
    ReportSvc *ReportService `inject:"true"`
}

func (j *ReportJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {
            Name: "daily-report",
            Expr: "0 8 * * *",    // every day at 8am
            Fn:   j.generateDailyReport,
        },
        {
            Name:            "hourly-sync",
            Expr:            "0 * * * *",  // every hour
            Fn:              j.syncExternalData,
            AllowConcurrent: false,         // skip if previous run still active
        },
    }
}

func (j *ReportJobs) generateDailyReport() {
    if err := j.ReportSvc.Generate(); err != nil {
        slog.Error("daily report failed", "err", err)
    }
}

func (j *ReportJobs) syncExternalData() {
    j.ReportSvc.Sync()
}
```

Register the provider as a component — the scheduling starter discovers it automatically:

```go
helix.Run(helix.App{
    Components: []any{
        &ReportService{},
        &ReportJobs{},
    },
})
```

## Job Configuration

```go
type Job struct {
    Name            string        // display name for logs
    Expr            string        // cron expression
    Fn              func()        // job function
    AllowConcurrent bool          // false = skip-if-busy (default)
}
```

### Cron Expression Format

Standard 5-field cron: `minute hour day-of-month month day-of-week`

| Expression | Meaning |
|------------|---------|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour |
| `0 8 * * *` | Every day at 8:00 AM |
| `0 8 * * 1` | Every Monday at 8:00 AM |
| `0 0 1 * *` | First day of every month |
| `*/15 * * * *` | Every 15 minutes |
| `0 9,17 * * 1-5` | 9 AM and 5 PM on weekdays |

## Concurrency Control

By default (`AllowConcurrent: false`), a job run is **skipped** if the previous one is still executing. This prevents queue buildup from slow jobs.

```go
{
    Name:            "data-sync",
    Expr:            "*/5 * * * *",  // every 5 min
    Fn:              j.sync,
    AllowConcurrent: false,          // skip if sync is still running
}
```

Set `AllowConcurrent: true` if the job is safe to run in parallel (e.g., independent tasks that don't share state).

## Panic Recovery

Panics inside job functions are caught automatically. A structured log entry is written and the scheduler continues running:

```log
{"level":"ERROR","msg":"job panicked","job":"daily-report","recover":"index out of range"}
```

Your application will not crash.

## Lifecycle

The scheduler implements `core.Lifecycle`:

- **OnStart** — resolves all `ScheduledJobProvider` components, registers their jobs, and starts the cron runner
- **OnStop** — waits for any running jobs to finish before the shutdown context deadline

```go
// Jobs are automatically started when the container starts:
container.Start()

// And stopped gracefully on shutdown:
container.Shutdown()
```

## Manual Scheduler Usage

Without `helix.Run`:

```go
import "github.com/enokdev/helix/scheduler"

s := scheduler.New()

s.Register(scheduler.Job{
    Name: "cleanup",
    Expr: "0 0 * * *",
    Fn: func() {
        cleanupExpiredSessions()
    },
})

if err := s.Start(); err != nil {
    log.Fatal(err)
}
defer s.Stop()
```

## Inspecting Registered Jobs

```go
entries := s.Entries()
for _, entry := range entries {
    fmt.Printf("Job: %s, Next: %s\n", entry.Name, entry.Next)
}
```

## Example: Multiple Job Providers

Each package can define its own job provider, keeping scheduling concerns local:

```go
// billing/jobs.go
type BillingJobs struct {
    helix.Component
    BillingSvc *BillingService `inject:"true"`
}

func (j *BillingJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {Name: "monthly-invoices", Expr: "0 6 1 * *", Fn: j.BillingSvc.GenerateInvoices},
        {Name: "payment-reminders", Expr: "0 9 * * *", Fn: j.BillingSvc.SendReminders},
    }
}

// notifications/jobs.go
type NotificationJobs struct {
    helix.Component
    NotifSvc *NotificationService `inject:"true"`
}

func (j *NotificationJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {Name: "digest-emails", Expr: "0 7 * * 1", Fn: j.NotifSvc.SendWeeklyDigest},
    }
}
```

All three jobs are discovered and registered automatically:

```go
helix.Run(helix.App{
    Components: []any{
        &BillingService{},
        &BillingJobs{},
        &NotificationService{},
        &NotificationJobs{},
    },
})
```
