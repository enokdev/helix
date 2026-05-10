# Injection de dépendances avancée

Ce guide couvre les patterns au-delà de l'injection de champ basique : mode Wire, injection d'interface, collecte de slices, chargement paresseux, portée prototype et injection de valeurs.

## Mode Wire (DI à la compilation)

### Pourquoi le mode Wire ?

Le `ReflectResolver` par défaut utilise la réflexion Go à l'exécution. Il fonctionne bien pour la plupart des apps, mais a un coût : les lookups de types et la population des champs se produisent au démarrage du processus. Pour les apps avec des centaines de composants ou des budgets de temps de démarrage stricts, le **mode Wire** génère le code de câblage à la compilation — zéro réflexion à l'exécution.

### Comment ça fonctionne

1. Écrivez vos composants normalement (mêmes marqueurs, mêmes tags `inject:"true"`)
2. Exécutez `helix generate wire`
3. Helix génère un fichier `wire_gen.go` qui appelle `RegisterWireSetup`
4. À l'exécution, le `WireResolver` délègue à cette fonction générée

```bash
helix generate wire
```

Sortie générée (`wire_gen.go`) :

```go
// Code généré par helix generate wire. NE PAS MODIFIER.
package main

import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/core"
    "your-app/user"
    "your-app/order"
)

func init() {
    helix.RegisterWireSetup(func(c *core.Container) error {
        userRepo := &user.Repository{}
        userSvc := &user.Service{Repo: userRepo}
        userCtrl := &user.Controller{Svc: userSvc}

        c.Register(userRepo)
        c.Register(userSvc)
        c.Register(userCtrl)
        return nil
    })
}
```

### Passer au mode Wire

```go
container := core.NewContainer(
    core.WithResolver(core.NewWireResolver()),
)
```

Ou via `helix.App` :

```go
helix.Run(helix.App{
    Mode: helix.ModeWire,
    // pas besoin de lister les Components — wire_gen.go les enregistre
})
```

### Quand utiliser le mode Wire

| Situation | Utiliser |
|-----------|----------|
| < 50 composants | ReflectResolver (défaut) |
| > 100 composants ou temps de démarrage critique | Mode Wire |
| CI/CD qui peut exécuter `helix generate` | Mode Wire |
| Intégration dans un système plus large | Mode Wire |

---

## Injection d'interface

### Une interface, une implémentation

C'est le cas commun. Enregistrez un type concret qui implémente l'interface, et injectez par type d'interface :

```go
type EmailSender interface {
    Send(to, subject, body string) error
}

type SMTPEmailSender struct {
    helix.Service
    host string
}

func (s *SMTPEmailSender) Send(to, subject, body string) error { ... }

// Enregistrement :
container.Register(&SMTPEmailSender{host: "smtp.example.com"})

// Injection par interface :
type UserService struct {
    helix.Service
    Email EmailSender `inject:"true"` // résolu vers *SMTPEmailSender
}
```

### Une interface, plusieurs implémentations

Utilisez `ComponentRegistration.ResolveAs` pour contrôler quelles interfaces un composant satisfait :

```go
type NotificationSender interface {
    Send(to, message string) error
}

type SMSSender struct{ helix.Service }
type SlackSender struct{ helix.Service }
type EmailNotificationSender struct{ helix.Service }

container.Register(core.ComponentRegistration{
    Component: &SMSSender{},
    ResolveAs: []reflect.Type{reflect.TypeOf((*NotificationSender)(nil)).Elem()},
})
container.Register(core.ComponentRegistration{
    Component: &SlackSender{},
    ResolveAs: []reflect.Type{reflect.TypeOf((*NotificationSender)(nil)).Elem()},
})
container.Register(core.ComponentRegistration{
    Component: &EmailNotificationSender{},
    ResolveAs: []reflect.Type{reflect.TypeOf((*NotificationSender)(nil)).Elem()},
})
```

### Collecter toutes les implémentations (`collect`)

Utilisez le tag `collect:""` pour injecter **tous les composants enregistrés** qui satisfont une interface sous forme de slice :

```go
type AlertService struct {
    helix.Service
    Senders []NotificationSender `collect:""`  // reçoit [*SMSSender, *SlackSender, *EmailNotificationSender]
}

func (a *AlertService) Alert(message string) {
    for _, s := range a.Senders {
        if err := s.Send("ops-team", message); err != nil {
            slog.Error("alerte échouée", "sender", fmt.Sprintf("%T", s), "err", err)
        }
    }
}
```

`collect:""` est particulièrement utile pour les architectures de type plugin : indicateurs de santé, écouteurs d'événements, middleware, exporteurs.

### Exclure un composant d'une interface

Utilisez `ExcludeFrom` pour empêcher un composant de satisfaire une interface qu'il correspondrait normalement :

```go
container.Register(core.ComponentRegistration{
    Component:   &InternalEmailSender{},
    ExcludeFrom: []reflect.Type{reflect.TypeOf((*NotificationSender)(nil)).Elem()},
})
// InternalEmailSender n'apparaîtra pas dans les slices []NotificationSender
```

---

## Portée : Singleton vs Prototype

### Singleton (défaut)

Une instance par conteneur — partagée par tous les composants qui la demandent. C'est le bon choix pour les services sans état, les repositories et les connexions.

```go
container.Register(&UserService{})  // singleton par défaut
```

### Prototype

Une **nouvelle instance** est créée chaque fois que le composant est résolu. Utilisez cela pour les objets avec état, par requête, ou à courte durée de vie.

```go
container.Register(core.ComponentRegistration{
    Component: &QueryBuilder{},
    Scope:     core.ScopePrototype,
})

// Chaque Resolve crée un nouveau *QueryBuilder :
var qb1 *QueryBuilder
container.Resolve(&qb1)

var qb2 *QueryBuilder
container.Resolve(&qb2)

// qb1 != qb2
```

Cas d'usage pratiques :
- Constructeurs de requêtes par requête
- Parsers / décodeurs avec état
- Instances de rate limiter par tenant

---

## Initialisation paresseuse

Un composant paresseux n'est pas résolu jusqu'à ce qu'il soit accédé pour la première fois. Utile pour les ressources coûteuses qui peuvent ne pas être nécessaires dans tous les chemins d'exécution :

```go
container.Register(core.ComponentRegistration{
    Component: &ReportGenerator{},
    Lazy:      true,
})

type AdminService struct {
    helix.Service
    Reports *ReportGenerator `inject:"true"` // pas résolu au démarrage
}

// *ReportGenerator n'est créé que lorsque AdminService.Reports est utilisé pour la première fois
```

Les composants paresseux participent toujours au cycle de vie (`OnStart` / `OnStop`) — mais `OnStart` est appelé au premier accès, pas au démarrage du conteneur.

---

## Injection de valeurs

Injectez des valeurs de configuration scalaires directement dans des champs de struct avec le tag `value:""` :

```go
type SMTPService struct {
    helix.Service
    Host     string `value:"email.smtp.host"`
    Port     int    `value:"email.smtp.port"`
    Username string `value:"email.smtp.username"`
}
```

Câblez la fonction de lookup lors de la création du conteneur :

```go
loader := config.NewLoader(...)

container := core.NewContainer(
    core.WithValueLookup(func(key string) (any, bool) {
        return loader.Lookup(key)
    }),
)
```

Avec `helix.Run`, le loader de config est câblé automatiquement — les tags `value:""` fonctionnent directement.

Fichier de config :

```yaml
email:
  smtp:
    host: "smtp.example.com"
    port: 587
    username: "noreply@example.com"
```

### Coercition de type

Le lookup de valeur doit retourner le bon type Go. Avec le loader de config Helix, les valeurs numériques et booléennes sont retournées dans leurs types natifs si le YAML est typé correctement :

```yaml
smtp:
  port: 587       # → int
  tls: true       # → bool
  timeout: 10s    # → string (à parser manuellement comme time.Duration)
```

---

## Résoudre tous les composants d'un type

`core.ResolveAll[T]` retourne tous les composants enregistrés qui implémentent `T` :

```go
// Résoudre toutes les implémentations de HealthIndicator :
indicators, err := core.ResolveAll[observability.HealthIndicator](container)
if err != nil {
    log.Fatal(err)
}

checker := observability.NewCompositeHealthChecker(indicators...)
```

C'est ainsi que le starter d'observabilité découvre les indicateurs de santé — et comment le starter de planification découvre les implémentations de `ScheduledJobProvider`.

---

## Scan de packages (`helix.App.Scan`)

Au lieu de lister chaque composant explicitement, déclarez quels packages scanner :

```go
helix.Run(helix.App{
    Scan: []string{
        "your-app/user",
        "your-app/order",
        "your-app/billing",
    },
})
```

Helix utilise l'analyse AST pour trouver tous les types exportés qui intègrent `helix.Service`, `helix.Controller`, `helix.Repository`, ou `helix.Component`, puis les enregistre automatiquement.

Combinez avec `Components` explicites pour les types qui nécessitent des arguments de constructeur :

```go
helix.Run(helix.App{
    Scan: []string{"your-app/..."},  // auto-découverte de tous les packages
    Components: []any{
        user.NewRepository(db),     // nécessite l'argument db
    },
})
```

---

## Inspection du graphe de dépendances

Inspectez le graphe résolu pour le débogage ou les outils :

```go
container.Register(&UserService{})
container.Register(&OrderService{})
container.Register(&UserRepository{})

graph := container.Graph()
// graph.Nodes — liste des noms de types de composants
// graph.Edges — map[string][]string de "composant → ses dépendances"

for node, deps := range graph.Edges {
    fmt.Printf("%s dépend de : %v\n", node, deps)
}
// UserService dépend de : [*UserRepository]
// OrderService dépend de : [*UserService, *UserRepository]
```

---

## Erreurs courantes

| Erreur | Cause | Correction |
|--------|-------|------------|
| `core.ErrNotFound` | Aucun composant enregistré ne correspond au type | Enregistrez le composant manquant |
| `core.ErrCyclicDep` | Cycle de dépendance A → B → A | Introduire une interface ou restructurer |
| `core.ErrUnresolvable` | Le composant peut être trouvé mais ne peut pas être construit | Vérifier les tags de champ, les pointeurs nil |
| `core.ErrShutdownTimeout` | Un `OnStop` a dépassé le budget | Augmenter `ShutdownTimeout` ou corriger le code bloquant |

### Inspecter une erreur de dépendance cyclique

```go
if errors.Is(err, core.ErrCyclicDep) {
    var cyclic *core.CyclicDepError
    errors.As(err, &cyclic)
    fmt.Println("cycle :", strings.Join(cyclic.Path, " → "))
    // ex : "cycle : *UserService → *OrderService → *UserService"
}
```
