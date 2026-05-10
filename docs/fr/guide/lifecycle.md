# Cycle de vie

Helix gère le cycle de vie de l'application — démarrage, arrêt et gestion des signaux — afin que vous puissiez vous connecter aux bons moments sans écrire de gestionnaires de signaux manuellement.

## L'interface Lifecycle

```go
// core/lifecycle.go
type Lifecycle interface {
    OnStart() error
    OnStop(ctx context.Context) error
}
```

Tout composant enregistré dans le conteneur DI qui implémente cette interface participe automatiquement au cycle de vie.

## Démarrage

`OnStart` est appelé après que toutes les dépendances sont résolues, dans l'**ordre de dépendance topologique**. Le `OnStart` d'un composant ne s'exécute qu'après que toutes ses dépendances ont démarré avec succès.

```go
type DatabaseConnection struct {
    helix.Component
    db *sql.DB
}

func (d *DatabaseConnection) OnStart() error {
    // Vérifier la connectivité avant que le serveur HTTP accepte des requêtes
    if err := d.db.Ping(); err != nil {
        return fmt.Errorf("base de données inaccessible : %w", err)
    }
    slog.Info("base de données connectée")
    return nil
}
```

Si un `OnStart` retourne une erreur, l'application se termine avant que le serveur HTTP ne démarre.

## Arrêt

`OnStop` est appelé dans l'**ordre inverse du démarrage** — le dernier composant à démarrer est le premier à s'arrêter. Tous les appels `OnStop` partagent un contexte avec deadline dérivé de `WithShutdownTimeout`.

```go
func (d *DatabaseConnection) OnStop(ctx context.Context) error {
    slog.Info("fermeture de la connexion à la base de données")
    return d.db.Close()
}
```

Le cycle de vie du serveur HTTP (géré par le starter web) draine les requêtes en vol avant l'arrêt de la base de données et des autres composants.

## Gestion des signaux

Avec `helix.Run`, Helix installe un gestionnaire de signaux pour `SIGTERM` et `SIGINT` (Ctrl+C). Sur signal :

1. Le serveur HTTP arrête d'accepter de nouvelles requêtes
2. Les requêtes en vol ont le temps de se terminer
3. `OnStop` est appelé sur tous les composants du cycle de vie en ordre inverse
4. L'application se termine proprement

## Timeout d'arrêt

Configurez le temps maximum autorisé pour l'arrêt :

```yaml
# config/application.yaml
helix:
  shutdown-timeout: 30s
```

Ou en code :

```go
helix.Run(helix.App{
    ShutdownTimeout: 45 * time.Second,
    Components:      []any{...},
})
```

La valeur par défaut est **30 secondes**. Si le `OnStop` d'un composant bloque au-delà du budget, `ErrShutdownTimeout` est retourné et le processus se termine.

## Patterns courants

### Worker en arrière-plan

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

### Pool de connexions à la base de données

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

### Connexion externe (Kafka, Redis, etc.)

```go
type KafkaProducer struct {
    helix.Component
    producer sarama.SyncProducer
    cfg      config.Loader `inject:"true"`
}

func (k *KafkaProducer) OnStart() error {
    brokers, _ := k.cfg.Lookup("kafka.brokers")
    // initialisation du producer...
    return nil
}

func (k *KafkaProducer) OnStop(ctx context.Context) error {
    return k.producer.Close()
}
```

## Ordre de démarrage et d'arrêt

Les composants démarrent dans l'ordre de dépendance topologique et s'arrêtent en ordre inverse. Avec :

```
UserController → UserService → UserRepository → DatabaseConnection
```

Séquence de démarrage :

```
1. DatabaseConnection.OnStart()   ← sans dépendances
2. UserRepository.OnStart()       ← dépend de DatabaseConnection
3. UserService.OnStart()          ← dépend de UserRepository
4. UserController.OnStart()       ← dépend de UserService
5. Le serveur HTTP commence à accepter des requêtes
```

Séquence d'arrêt (inverse) :

```
1. Le serveur HTTP arrête d'accepter de nouvelles requêtes, draine les requêtes en vol
2. UserController.OnStop()
3. UserService.OnStop()
4. UserRepository.OnStop()
5. DatabaseConnection.OnStop()    ← dernier à s'arrêter
```

Cela garantit que la base de données est encore disponible pendant que le serveur HTTP traite ses dernières requêtes.

## Que se passe-t-il si `OnStart` panique ?

Les panics dans `OnStart` sont capturés et traités comme des échecs de démarrage. L'application se termine avec une entrée de log :

```json
{"level":"ERROR","msg":"component startup panicked","component":"*DatabaseConnection","recover":"runtime error: invalid memory address"}
```

Les composants qui avaient déjà démarré avec succès auront leur `OnStop` appelé (le nettoyage est tenté).

## Que se passe-t-il si `OnStart` retourne une erreur ?

L'application se termine immédiatement :

```json
{"level":"ERROR","msg":"component failed to start","component":"*DatabaseConnection","error":"connection refused"}
```

Les composants qui avaient démarré avant le composant défaillant ont leur `OnStop` appelé.

## Arrêt partiel

Si `OnStop` retourne une erreur, elle est loguée mais l'arrêt continue — Helix appelle toujours `OnStop` sur tous les composants démarrés, quelle que soit la situation :

```json
{"level":"WARN","msg":"component failed to stop cleanly","component":"*CacheClient","error":"connection reset by peer"}
```

Cela empêche un seul composant bloqué de bloquer tout l'arrêt.

## Détails du timeout d'arrêt

Le budget d'arrêt est partagé entre tous les appels `OnStop`. Si le temps total dépasse `shutdown-timeout` :

```json
{"level":"ERROR","msg":"shutdown timed out","timeout":"30s","components_not_stopped":["*ReportWorker"]}
```

Le processus se termine quand même. Définissez le timeout assez élevé pour votre appel `OnStop` le plus lent (généralement le drainage des requêtes HTTP) :

```yaml
helix:
  shutdown-timeout: 45s  # définir à max(temps de drainage HTTP) + 10s de marge
```

## Utilisation directe du conteneur

Si vous gérez le conteneur vous-même (en dehors de `helix.Run`) :

```go
container := core.NewContainer()
container.Register(&DatabaseConnection{db: db})
container.Register(&MyService{})

// Démarrer tous les composants du cycle de vie dans l'ordre
if err := container.Start(); err != nil {
    log.Fatal(err)
}

// Arrêt (typiquement différé ou déclenché par signal)
defer container.Shutdown()
```
