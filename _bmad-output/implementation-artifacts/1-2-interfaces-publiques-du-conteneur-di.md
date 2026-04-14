# Story 1.2: Interfaces Publiques du Conteneur DI

Status: done

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

- [x] Tâche 1 : Créer `core/errors.go` — erreurs sentinelles (AC: #4)
  - [x] Déclarer `var ErrNotFound = errors.New("helix: not found")`
  - [x] Déclarer `var ErrCyclicDep = errors.New("helix: cyclic dependency")`
  - [x] Déclarer `var ErrUnresolvable = errors.New("helix: cannot resolve component")`
  - [x] Déclarer le type erreur structuré `CyclicDepError` avec champ `Path []string` et méthode `Error() string`
  - [x] Écrire `core/errors_test.go` — vérifier que les sentinelles sont des erreurs, que `CyclicDepError.Error()` formate le chemin

- [x] Tâche 2 : Créer `core/resolver.go` — interface Resolver + type DependencyGraph (AC: #2)
  - [x] Définir `type DependencyGraph struct` avec champs `Nodes []string` et `Edges map[string][]string`
  - [x] Définir l'interface `Resolver` avec `Register(component any) error`, `Resolve(target any) error`, `Graph() DependencyGraph`
  - [x] Écrire `core/resolver_test.go` — vérifier que `DependencyGraph{}` est zéro-valeur saine (pas de nil panic)

- [x] Tâche 3 : Créer `core/lifecycle.go` — interface Lifecycle (AC: #6)
  - [x] Définir `type Lifecycle interface { OnStart() error; OnStop() error }`
  - [x] Aucun test requis (interface pure — la conformité est vérifiée au compile-time par les implémenteurs)

- [x] Tâche 4 : Créer `core/options.go` — type Option + options fonctionnelles (AC: #3)
  - [x] Définir `type Option func(*Container)`
  - [x] Implémenter `WithResolver(r Resolver) Option` — injecte le resolver dans le container
  - [x] Écrire `core/options_test.go` — vérifier que `WithResolver` remplace bien le resolver du container

- [x] Tâche 5 : Créer `core/container.go` — struct Container + méthodes + NewContainer (AC: #1, #3)
  - [x] Définir `type Container struct { resolver Resolver }` (champ non-exporté)
  - [x] Implémenter `Register(component any) error` — délègue à `c.resolver.Register(component)` ; retourne `ErrUnresolvable` si resolver nil
  - [x] Implémenter `Resolve(target any) error` — délègue à `c.resolver.Resolve(target)` ; retourne `ErrUnresolvable` si resolver nil
  - [x] Implémenter `NewContainer(opts ...Option) *Container` — crée un container vide, applique les options
  - [x] Écrire `core/container_test.go` avec table-driven tests :
    - `Register` avec resolver nil → `ErrUnresolvable`
    - `Resolve` avec resolver nil → `ErrUnresolvable`
    - `NewContainer()` crée un container non-nil
    - `NewContainer(WithResolver(r))` injecte le resolver

- [x] Tâche 6 : Créer `core/registry.go` — type ComponentRegistration (prépare Story 1.3)
  - [x] Définir `type Scope string` avec constantes `ScopeSingleton Scope = "singleton"` et `ScopePrototype Scope = "prototype"`
  - [x] Définir `type ComponentRegistration struct { Component any; Scope Scope; Lazy bool }`
  - [x] Aucun test requis (types de données purs — utilisés par ReflectResolver en Story 1.3)

- [x] Tâche 7 : Valider les contraintes d'import et qualité (AC: #5, #7)
  - [x] Vérifier `go build ./core/...` sans erreur
  - [x] Vérifier `go test ./core/...` — tous les tests passent
  - [x] Confirmer manuellement qu'aucun `import "github.com/enokdev/helix/..."` n'apparaît dans `core/`

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

_Aucun blocage._

### Completion Notes List

- 6 fichiers créés dans `core/` : `errors.go`, `resolver.go`, `lifecycle.go`, `options.go`, `container.go`, `registry.go`
- 4 fichiers de test créés : `errors_test.go`, `resolver_test.go`, `options_test.go`, `container_test.go`
- `Container` implémenté comme struct avec délégation au `Resolver` via le pattern functional options
- `CyclicDepError` implémente `Unwrap()` pour compatibilité `errors.Is(err, ErrCyclicDep)`
- `core/` vérifié : zéro import vers d'autres packages Helix
- `go build ./...` ✅ — `go test ./...` ✅ (core: ok, zéro régression)
- `stubResolver` défini dans `options_test.go` (package `core`) — réutilisé dans `container_test.go` grâce à la co-location des tests

### File List

- `core/errors.go`
- `core/errors_test.go`
- `core/resolver.go`
- `core/resolver_test.go`
- `core/lifecycle.go`
- `core/options.go`
- `core/options_test.go`
- `core/container.go`
- `core/container_test.go`
- `core/registry.go`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (statut mis à jour)
- `_bmad-output/implementation-artifacts/1-2-interfaces-publiques-du-conteneur-di.md` (story file)

### Change Log

- 2026-04-14 : Implémentation des interfaces publiques du conteneur DI : Container, Resolver, Lifecycle, Option, Scope, ComponentRegistration, erreurs sentinelles + tests unitaires table-driven.

### Review Findings

- [x] [Review][Decision] ComponentRegistration.Scope zero value est `""` et non `ScopeSingleton` — **Résolu** : ajout de `NewComponentRegistration()` dans `core/registry.go` + `core/registry_test.go`. [core/registry.go]
- [x] [Review][Decision] `WithResolver(nil)` accepté silencieusement — **Résolu** : panic ajouté dans `WithResolver` + test `TestWithResolver_NilPanics`. [core/options.go]
- [x] [Review][Decision] `Register(nil)` / `Resolve(nil)` transférés au resolver sans garde — **Résolu** : garde nil ajouté dans `Container.Register` et `Container.Resolve` + tests table-driven mis à jour. [core/container.go]
- [x] [Review][Defer] `DependencyGraph.Edges` nil map — un consumer qui écrit dans la map retournée par `Graph()` sur une valeur zero panique ; aucune implémentation de `Graph()` n'existe encore (Story 1.3). [core/resolver.go] — deferred, pré-existant
- [x] [Review][Defer] `CyclicDepError` avec `Path` nil ou vide — `Error()` retourne `"helix: cyclic dependency: "` (message tronqué) ; aucune code path ne crée `CyclicDepError` dans cette story. [core/errors.go] — deferred, pré-existant
- [x] [Review][Defer] Absence de synchronisation sur `Container` — accès concurrent à `c.resolver` est une data race détectable par le race detector Go ; les exigences de concurrence ne sont pas encore définies. [core/container.go] — deferred, pré-existant
