# Injection de dépendances

Helix inclut un conteneur DI intégré avec injection automatique de champs, gestion du cycle de vie et deux modes de résolution.

## Le conteneur

```go
import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/core"
)

container := core.NewContainer()
```

### Options

```go
container := core.NewContainer(
    core.WithResolver(core.NewReflectResolver()),   // défaut
    core.WithShutdownTimeout(30 * time.Second),     // budget d'arrêt gracieux
    core.WithLogger(slog.Default()),
    core.WithValueLookup(func(key string) (any, bool) {
        // valeurs scalaires résolues via les tags value:"key"
        return os.LookupEnv(key)
    }),
)
```

## Marqueurs de composants

Intégrez un struct marqueur pour déclarer le rôle d'un composant. Helix les utilise pour auto-enregistrer les composants dans le sous-système approprié (HTTP, DI, etc.).

```go
type UserService struct {
    helix.Service     // couche logique métier
}

type UserController struct {
    helix.Controller  // handler HTTP — auto-enregistré dans le routeur
}

type UserRepository struct {
    helix.Repository  // couche d'accès aux données
}

type CacheManager struct {
    helix.Component   // composant générique
}
```

## Enregistrement des composants

```go
container.Register(&UserRepository{})
container.Register(&UserService{})
container.Register(&UserController{})
```

L'ordre d'enregistrement n'a pas d'importance — Helix résout le graphe de dépendances automatiquement.

### Enregistrement avancé

Utilisez `ComponentRegistration` pour un contrôle précis :

```go
container.Register(core.ComponentRegistration{
    Component:   &UserService{},
    Scope:       core.ScopePrototype,    // nouvelle instance à chaque résolution
    Lazy:        true,                   // ne pas résoudre avant la première utilisation
    ResolveAs:   []reflect.Type{reflect.TypeOf((*UserReader)(nil)).Elem()},
    ExcludeFrom: []reflect.Type{reflect.TypeOf((*Auditable)(nil)).Elem()},
})
```

| Champ | Description |
|-------|-------------|
| `Scope` | `ScopeSingleton` (défaut) ou `ScopePrototype` |
| `Lazy` | Retarder la résolution jusqu'à la première utilisation |
| `ResolveAs` | Limiter les types d'interfaces satisfaits |
| `ExcludeFrom` | Retirer de certaines résolutions d'interface |

## Injection de champs

Annotez les champs de struct avec `inject:"true"` :

```go
type OrderService struct {
    helix.Service
    UserSvc *UserService       `inject:"true"` // résolu par type
    PaySvc  *PaymentService    `inject:"true"`
    Logger  *slog.Logger       `inject:"true"`
}
```

### Injection de valeurs

Injectez des valeurs scalaires depuis une fonction de lookup :

```go
type EmailService struct {
    helix.Service
    SMTPHost string `value:"smtp.host"`  // résolu via WithValueLookup
    SMTPPort int    `value:"smtp.port"`
}
```

## Injection par slice avec `collect`

Utilisez `collect:""` pour injecter **tous les composants enregistrés** qui implémentent une interface :

```go
type HealthIndicator interface {
    Check(ctx context.Context) ComponentHealth
}

// Plusieurs implémentations enregistrées :
container.Register(&DatabaseIndicator{})
container.Register(&CacheIndicator{})
container.Register(&ExternalAPIIndicator{})

// Collectées automatiquement dans une slice :
type HealthService struct {
    helix.Service
    Indicators []HealthIndicator `collect:""`
    // reçoit [*DatabaseIndicator, *CacheIndicator, *ExternalAPIIndicator]
}
```

C'est ainsi qu'Helix lui-même découvre les indicateurs de santé, les fournisseurs de jobs planifiés et les écouteurs d'événements.

## Initialisation lazy

Ajoutez le tag `lazy:""` pour retarder la résolution jusqu'au premier accès :

```go
type AdminService struct {
    helix.Service
    // Créé seulement lors du premier accès — pas au démarrage du conteneur
    Reports *ReportGenerator `inject:"true" lazy:""`
}
```

Utile pour les ressources coûteuses (générateurs de rapports, gros caches) qui ne sont nécessaires que sur certains chemins d'exécution.

## Injection d'interface

Enregistrez un type concret et injectez par interface :

```go
type NotificationService interface {
    Notify(to, message string) error
}

type SlackNotifier struct {
    helix.Service
    webhookURL string
}

func (s *SlackNotifier) Notify(to, message string) error { ... }

// Enregistrer le concret :
container.Register(&SlackNotifier{webhookURL: "https://hooks.slack.com/..."})

// Injecter par interface :
type AlertService struct {
    helix.Service
    Notifier NotificationService `inject:"true"` // résolu vers *SlackNotifier
}
```

Pour plusieurs implémentations de la même interface, voir [DI avancé](/fr/guide/advanced-di).

## Résolution des composants

```go
// Résoudre un seul composant
var svc *UserService
if err := container.Resolve(&svc); err != nil {
    log.Fatal(err)
}

// Résoudre tous les composants implémentant une interface
providers, err := core.ResolveAll[HealthIndicator](container)
```

## Modes de résolution

### Mode Reflect (défaut)

Utilise la réflexion Go au runtime. Zéro configuration — adapté à la plupart des applications.

```go
container := core.NewContainer(
    core.WithResolver(core.NewReflectResolver()),
)
```

### Mode Wire (compile-time)

Génère le câblage DI à la compilation via `helix generate`. Zéro overhead runtime.

```go
container := core.NewContainer(
    core.WithResolver(core.NewWireResolver()),
)
```

Exécutez la génération de code :

```bash
helix generate
```

## Cycle de vie

Les composants implémentant `core.Lifecycle` participent au cycle de vie de l'application :

```go
type core.Lifecycle interface {
    OnStart() error
    OnStop(ctx context.Context) error
}
```

```go
type DatabaseConnection struct {
    helix.Component
    db *sql.DB
}

func (d *DatabaseConnection) OnStart() error {
    return d.db.Ping()
}

func (d *DatabaseConnection) OnStop(ctx context.Context) error {
    return d.db.Close()
}
```

**Ordre de démarrage** — tri topologique par graphe de dépendances, en préservant l'ordre d'enregistrement pour les composants de même niveau.

**Ordre d'arrêt** — inverse du démarrage, avec un contexte de deadline partagé depuis `WithShutdownTimeout`.

```go
// Démarrer tous les composants lifecycle enregistrés
if err := container.Start(); err != nil {
    log.Fatal(err)
}

// Arrêter tous (appelé automatiquement sur SIGTERM/SIGINT avec helix.Run)
if err := container.Shutdown(); err != nil {
    log.Fatal(err)
}
```

## Graphe de dépendances

Inspectez le graphe de dépendances résolu de manière programmatique :

```go
graph := container.Graph()
// graph.Nodes — liste des noms de types de composants
// graph.Edges — map[string][]string des dépendances

for node, deps := range graph.Edges {
    fmt.Printf("%s dépend de : %v\n", node, deps)
}
```

## Erreurs courantes

| Erreur | Cause | Correction |
|--------|-------|-----------|
| `core.ErrNotFound` | Aucun composant correspond au type cible | Enregistrer le composant manquant |
| `core.ErrCyclicDep` | Dépendance circulaire A → B → A | Introduire une interface ou restructurer |
| `core.ErrUnresolvable` | Composant trouvé mais ne peut pas être construit | Vérifier les tags, les pointeurs nil |
| `core.ErrShutdownTimeout` | Un `OnStop` a dépassé le budget | Augmenter `ShutdownTimeout` ou corriger le code bloquant |

### Diagnostiquer un cycle

```go
if errors.Is(err, core.ErrCyclicDep) {
    var cyclic *core.CyclicDepError
    errors.As(err, &cyclic)
    fmt.Println("chemin du cycle :", strings.Join(cyclic.Path, " → "))
    // "chemin du cycle : *UserService → *OrderService → *UserService"
}
```

## Utiliser `helix.Run`

Dans la plupart des applications, vous ne touchez pas directement le conteneur — `helix.Run` le gère :

```go
helix.Run(helix.App{
    Components: []any{
        &UserRepository{},
        &UserService{},
        &UserController{},
    },
    ShutdownTimeout: 30 * time.Second,
})
```

`helix.Run` crée le conteneur, enregistre tous les composants, active les starters, démarre le serveur HTTP et gère SIGTERM/SIGINT gracieusement.

Pour le mode Wire, l'injection en compile-time et tous les patterns avancés, voir [DI avancé](/fr/guide/advanced-di).
