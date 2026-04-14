# Story 1.2: Interfaces Publiques du Conteneur DI

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

En tant que **développeur utilisant Helix**,
Je veux une interface `Container` claire pour enregistrer et résoudre des dépendances,
Afin de comprendre le contrat du DI sans lire l'implémentation.

## Acceptance Criteria

1. **Given** le package `core/` importé
   **When** un développeur consulte les types exportés
   **Then** le type `Container` (struct) expose les méthodes `Register(component any) error` et `Resolve(target any) error`

2. **And** l'interface `Resolver` est définie avec `Register(component any) error`, `Resolve(target any) error`, `Graph() DependencyGraph`

3. **And** `NewContainer(opts ...Option) *Container` crée un container fonctionnel (compilable, resolver configurable via options)

4. **And** les erreurs sentinelles `ErrNotFound`, `ErrCyclicDep`, `ErrUnresolvable` sont exportées dans `core/errors.go`

5. **And** `core/` n'importe aucun autre package Helix (`go build ./...` ne doit détecter aucun import interne à Helix)

6. **And** l'interface `Lifecycle` est définie avec `OnStart() error` et `OnStop() error` dans `core/lifecycle.go`

7. **And** `go test ./core/...` s'exécute sans erreur (tests unitaires de contrat)

## Tasks / Subtasks

- [ ] Tâche 1 : Créer `core/errors.go` — erreurs sentinelles (AC: #4)
  - [ ] Déclarer `var ErrNotFound = errors.New("helix: not found")`
  - [ ] Déclarer `var ErrCyclicDep = errors.New("helix: cyclic dependency")`
  - [ ] Déclarer `var ErrUnresolvable = errors.New("helix: cannot resolve component")`
  - [ ] Déclarer le type erreur structuré `CyclicDepError` avec champ `Path []string` et méthode `Error() string`
  - [ ] Écrire `core/errors_test.go` — vérifier que les sentinelles sont des erreurs, que `CyclicDepError.Error()` formate le chemin

- [ ] Tâche 2 : Créer `core/resolver.go` — interface Resolver + type DependencyGraph (AC: #2)
  - [ ] Définir `type DependencyGraph struct` avec champs `Nodes []string` et `Edges map[string][]string`
  - [ ] Définir l'interface `Resolver` avec `Register(component any) error`, `Resolve(target any) error`, `Graph() DependencyGraph`
  - [ ] Écrire `core/resolver_test.go` — vérifier que `DependencyGraph{}` est zéro-valeur saine (pas de nil panic)

- [ ] Tâche 3 : Créer `core/lifecycle.go` — interface Lifecycle (AC: #6)
  - [ ] Définir `type Lifecycle interface { OnStart() error; OnStop() error }`
  - [ ] Aucun test requis (interface pure — la conformité est vérifiée au compile-time par les implémenteurs)

- [ ] Tâche 4 : Créer `core/options.go` — type Option + options fonctionnelles (AC: #3)
  - [ ] Définir `type Option func(*Container)`
  - [ ] Implémenter `WithResolver(r Resolver) Option` — injecte le resolver dans le container
  - [ ] Écrire `core/options_test.go` — vérifier que `WithResolver` remplace bien le resolver du container

- [ ] Tâche 5 : Créer `core/container.go` — struct Container + méthodes + NewContainer (AC: #1, #3)
  - [ ] Définir `type Container struct { resolver Resolver }` (champ non-exporté)
  - [ ] Implémenter `Register(component any) error` — délègue à `c.resolver.Register(component)` ; retourne `ErrUnresolvable` si resolver nil
  - [ ] Implémenter `Resolve(target any) error` — délègue à `c.resolver.Resolve(target)` ; retourne `ErrUnresolvable` si resolver nil
  - [ ] Implémenter `NewContainer(opts ...Option) *Container` — crée un container vide, applique les options
  - [ ] Écrire `core/container_test.go` avec table-driven tests :
    - `Register` avec resolver nil → `ErrUnresolvable`
    - `Resolve` avec resolver nil → `ErrUnresolvable`
    - `NewContainer()` crée un container non-nil
    - `NewContainer(WithResolver(r))` injecte le resolver

- [ ] Tâche 6 : Créer `core/registry.go` — type ComponentRegistration (prépare Story 1.3)
  - [ ] Définir `type Scope string` avec constantes `ScopeSingleton Scope = "singleton"` et `ScopePrototype Scope = "prototype"`
  - [ ] Définir `type ComponentRegistration struct { Component any; Scope Scope; Lazy bool }`
  - [ ] Aucun test requis (types de données purs — utilisés par ReflectResolver en Story 1.3)

- [ ] Tâche 7 : Valider les contraintes d'import et qualité (AC: #5, #7)
  - [ ] Vérifier `go build ./core/...` sans erreur
  - [ ] Vérifier `go test ./core/...` — tous les tests passent
  - [ ] Confirmer manuellement qu'aucun `import "github.com/enokdev/helix/..."` n'apparaît dans `core/`

## Dev Notes

### Périmètre Strict de cette Story

**IMPORTANT :** Story 1.2 = INTERFACES & TYPES PUBLICS UNIQUEMENT. Aucune implémentation de résolution réelle :
- `ReflectResolver` → Story 1.3 (`core/internal/reflect_resolver.go`)
- Détection de cycles → Story 1.4 (`core/internal/graph.go`)
- Scopes Prototype / Lazy → Story 1.5
- Lifecycle `OnStart`/`OnStop` hooks → Story 1.6
- `helix.Run()` + auto-scan → Story 1.7

**Ne pas implémenter** ce qui appartient aux stories suivantes. Si une tâche semble nécessiter de la reflection ou du parcours de graphe, STOP — c'est Story 1.3+.

### Structure Cible de `core/` après cette Story

```
core/
├── container.go          # struct Container + Register/Resolve + NewContainer
├── resolver.go           # interface Resolver + type DependencyGraph
├── lifecycle.go          # interface Lifecycle
├── registry.go           # types Scope, ComponentRegistration
├── options.go            # type Option + WithResolver
├── errors.go             # ErrNotFound, ErrCyclicDep, ErrUnresolvable, CyclicDepError
├── container_test.go     # table-driven tests Container
├── resolver_test.go      # tests DependencyGraph zero-value
├── options_test.go       # tests WithResolver
├── errors_test.go        # tests sentinelles et CyclicDepError
└── doc.go                # ← déjà créé par Story 1.1, NE PAS MODIFIER
└── internal/             # ← vide pour l'instant, ReflectResolver Story 1.3
```

### Règles de Naming Critiques (architecture.md)

```go
// INTERFACES — sans préfixe I, sans suffixe Interface
type Resolver interface { ... }    // CORRECT
type Lifecycle interface { ... }   // CORRECT
type IResolver interface { ... }   // INTERDIT

// CONSTRUCTEURS — New + type retourné
func NewContainer(opts ...Option) *Container { ... }  // CORRECT
func CreateContainer() *Container { ... }             // INTERDIT

// ERREURS SENTINELLES — préfixe Err
var ErrNotFound  = errors.New("helix: not found")    // CORRECT
var NotFoundErr  = errors.New("...")                  // INTERDIT

// TYPES ERREUR — suffixe Error
type CyclicDepError struct { Path []string }          // CORRECT
type CyclicDependencyErr struct { ... }               // INTERDIT
```

### Design de `Container` — Struct, pas Interface

L'AC dit "interface Container" au sens sémantique (contrat public), mais en Go, `Container` est une **struct** avec méthodes exportées. Ce choix :
- Permet `NewContainer() *Container` sans interface vide
- Les stories 1.3-1.7 ajouteront des méthodes sur cette struct
- Si une interface `Container` est nécessaire (testutil MockBean), elle sera ajoutée en Story 1.7

```go
// core/container.go
type Container struct {
    resolver Resolver  // injecté via WithResolver option
}

func (c *Container) Register(component any) error {
    if c.resolver == nil {
        return ErrUnresolvable
    }
    return c.resolver.Register(component)
}

func (c *Container) Resolve(target any) error {
    if c.resolver == nil {
        return ErrUnresolvable
    }
    return c.resolver.Resolve(target)
}

func NewContainer(opts ...Option) *Container {
    c := &Container{}
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

### Design de `Resolver` — Interface Abstraite

```go
// core/resolver.go
type Resolver interface {
    Register(component any) error
    Resolve(target any) error
    Graph() DependencyGraph
}

type DependencyGraph struct {
    Nodes []string            // noms des types enregistrés
    Edges map[string][]string // dépendances : type → []types requis
}
```

### Design de `Option` — Functional Options Pattern

```go
// core/options.go
type Option func(*Container)

func WithResolver(r Resolver) Option {
    return func(c *Container) {
        c.resolver = r
    }
}
```

Deux autres options (`WithMode`, `WithScan`) seront ajoutées en Story 1.7 quand `helix.Run()` est implémenté. Ne pas anticiper ici.

### Design de `Lifecycle`

```go
// core/lifecycle.go
type Lifecycle interface {
    OnStart() error
    OnStop() error
}
```

L'appel de ces hooks est géré par Story 1.6. Story 1.2 ne fait que définir l'interface.

### Design des Erreurs

```go
// core/errors.go
import "errors"
import "strings"

var (
    ErrNotFound     = errors.New("helix: not found")
    ErrCyclicDep    = errors.New("helix: cyclic dependency")
    ErrUnresolvable = errors.New("helix: cannot resolve component")
)

// CyclicDepError enrichit ErrCyclicDep avec le chemin du cycle
type CyclicDepError struct {
    Path []string
}

func (e *CyclicDepError) Error() string {
    return "helix: cyclic dependency: " + strings.Join(e.Path, " → ")
}

func (e *CyclicDepError) Unwrap() error {
    return ErrCyclicDep
}
```

`CyclicDepError.Unwrap()` permet `errors.Is(err, ErrCyclicDep)` — toujours fournir `Unwrap()` pour les types erreur wrappant des sentinelles.

### Conventions de Test

```go
// Nommage : Test{Type}_{Method}
func TestContainer_Register(t *testing.T) { ... }
func TestContainer_Resolve(t *testing.T)  { ... }
func TestContainer_NewContainer(t *testing.T) { ... }

// Table-driven obligatoire pour cas multiples
func TestContainer_Register(t *testing.T) {
    tests := []struct {
        name      string
        resolver  Resolver
        component any
        wantErr   error
    }{
        {name: "resolver nil retourne ErrUnresolvable", resolver: nil, component: &struct{}{}, wantErr: ErrUnresolvable},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c := &Container{resolver: tt.resolver}
            err := c.Register(tt.component)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

**Pas de `testify`** à ce stade — pas de dépendances ajoutées dans cette story. Utiliser `testing` stdlib uniquement.

### Contrainte d'Import Core (CRITIQUE)

`core/` doit avoir **zéro import** vers d'autres packages Helix :
```go
// INTERDIT dans tout fichier de core/
import "github.com/enokdev/helix/config"
import "github.com/enokdev/helix/web"
import "github.com/enokdev/helix/data"
// etc.
```

Imports autorisés dans `core/` :
- `errors` (stdlib)
- `strings` (stdlib)
- `fmt` (stdlib)
- Autres packages stdlib uniquement

### Wrapping d'Erreurs — Convention

```go
// CORRECT — wrapping avec contexte package: action:
return fmt.Errorf("core: register %T: %w", component, ErrUnresolvable)

// MOINS BIEN — retourne directement la sentinelle
return ErrUnresolvable
```

Pour Story 1.2, retourner directement les sentinelles est acceptable car il n'y a pas encore de contexte d'action (pas de resolver). Le wrapping contextuel sera introduit en Story 1.3.

### Learnings Story 1.1

- Module : `github.com/enokdev/helix`, go 1.21 — **ne pas modifier go.mod dans cette story**
- `core/doc.go` existe déjà (package core) — **ne pas modifier**
- `go build ./...` et `go test ./...` passent — maintenir ces invariants
- `.golangci.yml` activé avec `deadcode` à la place de `unused` — les types définis mais non utilisés ailleurs sont normaux dans une story d'interfaces pures

### Project Structure Notes

- Tous les fichiers sont dans `core/` (pas de sous-package pour cette story)
- `core/internal/` reste vide — ReflectResolver créé en Story 1.3
- Tests co-localisés dans `core/*_test.go` — JAMAIS dans `core/test/`
- `doc.go` existant : **ne pas toucher**

### Références

- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "1. Architecture Duale DI — Couche Resolver abstraite"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Patterns d'Implémentation — 1. Naming Patterns"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Patterns d'Implémentation — 2. Organisation des Packages"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Patterns d'Implémentation — 3. Interfaces Publiques"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Patterns d'Implémentation — 4. Gestion d'Erreurs"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Structure du Projet — Arborescence Complète"]
- [Source: `_bmad-output/planning-artifacts/epics.md` — Story 1.2 Acceptance Criteria]
- [Source: `_bmad-output/implementation-artifacts/1-1-initialisation-du-projet-structure-de-base.md` — Story 1.1 completed, learnings]

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.6 (claude-sonnet-4-6)

### Debug Log References

### Completion Notes List

### File List
