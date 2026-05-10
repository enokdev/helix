# Planification (Cron)

Helix inclut un planificateur basé sur cron avec un arrêt gracieux, une récupération des panics et un contrôle de la concurrence — activé automatiquement quand `github.com/robfig/cron/v3` est dans votre `go.mod`.

## Définir des jobs

Implémentez `scheduler.ScheduledJobProvider` pour enregistrer des jobs cron :

```go
import "github.com/enokdev/helix/scheduler"

type ReportJobs struct {
    helix.Component
    ReportSvc *ReportService `inject:"true"`
}

func (j *ReportJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {
            Name: "rapport-quotidien",
            Expr: "0 8 * * *",    // tous les jours à 8h
            Fn:   j.generateDailyReport,
        },
        {
            Name:            "sync-horaire",
            Expr:            "0 * * * *",  // toutes les heures
            Fn:              j.syncExternalData,
            AllowConcurrent: false,         // passer si l'exécution précédente est encore active
        },
    }
}

func (j *ReportJobs) generateDailyReport() {
    if err := j.ReportSvc.Generate(); err != nil {
        slog.Error("échec rapport quotidien", "err", err)
    }
}

func (j *ReportJobs) syncExternalData() {
    j.ReportSvc.Sync()
}
```

Enregistrez le provider comme composant — le starter de planification le découvre automatiquement :

```go
helix.Run(helix.App{
    Components: []any{
        &ReportService{},
        &ReportJobs{},
    },
})
```

## Configuration des jobs

```go
type Job struct {
    Name            string   // nom d'affichage pour les logs
    Expr            string   // expression cron
    Fn              func()   // fonction du job
    AllowConcurrent bool     // false = skip-si-occupé (défaut)
}
```

### Format des expressions cron

Cron standard à 5 champs : `minute heure jour-du-mois mois jour-de-la-semaine`

| Expression | Signification |
|------------|--------------|
| `* * * * *` | Toutes les minutes |
| `0 * * * *` | Toutes les heures |
| `0 8 * * *` | Tous les jours à 8h00 |
| `0 8 * * 1` | Tous les lundis à 8h00 |
| `0 0 1 * *` | Premier jour de chaque mois |
| `*/15 * * * *` | Toutes les 15 minutes |
| `0 9,17 * * 1-5` | 9h et 17h les jours ouvrés |

## Contrôle de la concurrence

Par défaut (`AllowConcurrent: false`), une exécution de job est **ignorée** si la précédente est encore en cours. Cela évite l'accumulation de queue avec des jobs lents.

```go
{
    Name:            "sync-données",
    Expr:            "*/5 * * * *",  // toutes les 5 min
    Fn:              j.sync,
    AllowConcurrent: false,          // ignorer si sync est encore en cours
}
```

## Helpers de jobs

### `scheduler.WrapError`

Convertit une fonction retournant `error` en signature `func()` attendue par `Job.Fn` :

```go
func (j *ReportJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {
            Name: "rapport-quotidien",
            Expr: "0 8 * * *",
            // WrapError logue l'erreur et retourne — pas de panic, pas de crash
            Fn: scheduler.WrapError("rapport-quotidien", func() error {
                return j.ReportSvc.Generate()
            }),
        },
    }
}
```

### `scheduler.WrapSkipIfBusy`

Ignorer une exécution de job si la précédente est encore en cours :

```go
{
    Name:            "processeur-batch",
    Expr:            "*/2 * * * *",
    AllowConcurrent: true,
    Fn:              scheduler.WrapSkipIfBusy(j.processBatch),
}
```

## Timezone dans les expressions cron

Par défaut, les expressions cron utilisent UTC. Préfixez l'expression avec une directive `CRON_TZ` pour un fuseau horaire local :

```go
{
    Name: "rapport-matinal",
    Expr: "CRON_TZ=Europe/Paris 0 8 * * *",  // 8h heure de Paris
    Fn:   j.generateReport,
}

{
    Name: "ouverture-marché-ny",
    Expr: "CRON_TZ=America/New_York 30 9 * * 1-5",  // 9h30 EST/EDT, jours ouvrés
    Fn:   j.checkMarketOpen,
}
```

## Récupération des panics

Les panics dans les fonctions de job sont capturés automatiquement. Une entrée de log structurée est écrite et le planificateur continue à tourner :

```log
{"level":"ERROR","msg":"job paniqué","job":"rapport-quotidien","recover":"index out of range"}
```

Votre application ne crashera pas.

## Erreurs

| Erreur | Cause |
|--------|-------|
| `scheduler.ErrInvalidCron` | Expression cron malformée |
| `scheduler.ErrInvalidJob` | Job sans Nom ou Fn |
| `scheduler.ErrDuplicateJob` | Un job avec ce nom est déjà enregistré |
| `scheduler.ErrJobNotFound` | `Unregister` appelé avec un nom inconnu |

```go
if err := s.Register(job); err != nil {
    switch {
    case errors.Is(err, scheduler.ErrInvalidCron):
        log.Fatalf("expression cron invalide %q : %v", job.Expr, err)
    case errors.Is(err, scheduler.ErrDuplicateJob):
        log.Fatalf("le job %q est déjà enregistré", job.Name)
    }
}
```

## Cycle de vie

Le planificateur implémente `core.Lifecycle` :

- **OnStart** — résout tous les composants `ScheduledJobProvider`, enregistre leurs jobs, et démarre le runner cron
- **OnStop** — attend que les jobs en cours terminent avant la deadline du contexte de shutdown

## Tester les jobs planifiés

Testez la fonction du job directement — pas besoin d'exécuter le planificateur :

```go
func TestDailyReport(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &MockReportService{},
            &ReportJobs{},
        ),
    )

    jobs := helix.GetBean[*ReportJobs](app)

    // Appeler la fonction du job directement :
    jobs.generateDailyReport()

    // Vérifier les effets de bord :
    mock := helix.GetBean[*MockReportService](app)
    if !mock.Generated {
        t.Fatal("rapport attendu généré")
    }
}
```

## Exemple : Plusieurs fournisseurs de jobs

Chaque package peut définir son propre fournisseur de jobs, en gardant les préoccupations de planification locales :

```go
// billing/jobs.go
type BillingJobs struct {
    helix.Component
    BillingSvc *BillingService `inject:"true"`
}

func (j *BillingJobs) Jobs() []scheduler.Job {
    return []scheduler.Job{
        {Name: "factures-mensuelles", Expr: "0 6 1 * *", Fn: j.BillingSvc.GenerateInvoices},
        {Name: "rappels-paiement",   Expr: "0 9 * * *",  Fn: j.BillingSvc.SendReminders},
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

Tous les jobs sont découverts et enregistrés automatiquement.
