# Helix Framework - Documentation pour Gemini

Ce document fournit le contexte et les instructions nécessaires pour travailler sur le framework **Helix**.

## Présentation du projet

Helix est un framework backend Go inspiré de **Spring Boot**, conçu pour minimiser le boilerplate tout en restant idiomatique. Il repose sur un système d'injection de dépendances (DI), un routage déclaratif et une configuration automatique via des "starters".

### Architecture et Concepts Clés

- **Composants :** Les composants sont des structures Go qui intègrent des marqueurs Helix par composition anonyme :
  - `helix.Service` : Logique métier.
  - `helix.Controller` : Points d'entrée HTTP.
  - `helix.Repository` : Accès aux données.
  - `helix.Component` : Composant générique.
  - `helix.ErrorHandler` : Gestion centralisée des erreurs.
  - `helix.SecurityConfigurer` : Configuration de la sécurité globale.

- **Injection de Dépendances (DI) :**
  - Les dépendances sont injectées automatiquement dans les champs marqués par le tag `inject:"true"`.
  - Deux modes de résolution : **Reflect** (runtime via réflexion, par défaut) et **Wire** (génération de code au compile-time).

- **Couche Web :**
  - Utilise **Fiber** sous le capot.
  - Routage par convention (`Index`, `Show`, `Create`, `Update`, `Delete`) ou par directives (`//helix:route GET /path`).
  - Support des guards (`//helix:guard`) et des intercepteurs (`//helix:interceptor`).

- **Starters :** Auto-configuration modulaire détectée selon le `go.mod` et la configuration :
  - `web` : Serveur HTTP Fiber.
  - `data` : Persistance (support GORM).
  - `security` : JWT et RBAC.
  - `observability` : Métriques (Prometheus), Tracing (OTel), Health checks.
  - `scheduling` : Tâches cron.

---

## Construction et Tests

### Commandes Framework

Pour développer le framework lui-même :

```bash
# Télécharger les dépendances
go mod tidy

# Lancer tous les tests
go test ./...

# Compiler le framework
go build ./...

# Lancer le linter (si installé)
golangci-lint run
```

### Exemples d'utilisation

Pour explorer les capacités du framework, plusieurs exemples sont disponibles dans le dossier `examples/` :

```bash
# Exemple CRUD complet
go run ./examples/crud-api

# Exemple avec sécurité JWT/RBAC
go run ./examples/secured-api

# Exemple Zero-Config
go run ./examples/zero-config
```

---

## Conventions de Développement

### Déclaration de Composants

```go
type MyService struct {
    helix.Service
    Repo *MyRepository `inject:"true"`
}
```

### Contrôleurs et Routage

- Le nom du contrôleur doit se terminer par `Controller` (ex: `UserController`).
- Par convention, `UserController` sera monté sur `/users`.
- Les routes sont dérivées des noms de méthodes ou de directives :

```go
type UserController struct {
    helix.Controller
}

// Index répond à GET /users
func (c *UserController) Index() []User { ... }

// Route personnalisée
//helix:route POST /users/search
func (c *UserController) Search(req SearchRequest) []User { ... }
```

### Gestion des Erreurs

Utilisez `helix.ErrorHandler` pour capturer et transformer les erreurs Go en réponses HTTP avec codes d'état.

### Tests d'Intégration

Utilisez `testutil.App` pour créer un environnement de test complet avec injection de dépendances.

---

## Structure du Code

- `/core` : Moteur de DI et gestion du cycle de vie.
- `/web` : Logique de routage, binding et adaptateurs Fiber.
- `/config` : Chargement de configuration (Viper) et profils (YAML).
- `/data` : Abstraction de la couche de données.
- `/starter` : Logique d'auto-configuration.
- `/cli` : Implémentation de l'outil en ligne de commande `helix`.
- `/docs` : Documentation détaillée par couche (en français).



## Commit Message Format

**ALWAYS** use conventional commits format. **NEVER** add a `Co-Authored-By` line or any other trailer.

Format: `<type>(<scope>): <description>`

Examples:
```
feat(core): implement ReflectResolver with singleton scope
fix(web): correct convention routing for DELETE methods
test(data): add integration tests for GormRepository
refactor(config): extract ENV > YAML > DEFAULT priority logic
```

Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`
Scopes: `core`, `web`, `data`, `config`, `starter`, `observability`, `scheduler`, `cli`

## GitHub Project Integration

**Repository:** `enokdev/helix` — **Project:** `https://github.com/orgs/enokdev/projects/1`

### Verifie que le CI passe
1. Verify that the CI passes , if not, investigate and fix before closing the issue.
