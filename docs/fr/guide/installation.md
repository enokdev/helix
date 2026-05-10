# Installation

## Prérequis

- **Go 1.21+** — Helix utilise les génériques et le package standard `log/slog`
- **Git**
- **Node.js 18+** — uniquement si vous souhaitez construire cette documentation localement

Vérifiez votre version Go :

```bash
go version
# go version go1.21.0 ou plus récent
```

## Installer le CLI

Le CLI Helix génère des projets, génère du code et gère les migrations de base de données.

```bash
go install github.com/enokdev/helix/cmd/helix@latest
```

Vérification :

```bash
helix --version
```

::: details `helix: command not found` après l'installation

`go install` place le binaire dans `$GOPATH/bin` (défaut : `~/go/bin`). Si ce répertoire n'est pas dans votre `PATH`, le shell ne trouve pas `helix`.

```bash
# À ajouter dans ~/.zshrc ou ~/.bashrc
export PATH="$PATH:$HOME/go/bin"

# Appliquer immédiatement
source ~/.zshrc
```

:::

::: details `compile: version "goX.Y.Z" does not match go tool version "goA.B.C"`

Cela arrive quand le binaire `go` et `GOROOT` pointent vers des installations Go différentes — fréquent lors de l'utilisation d'un gestionnaire de versions Go (govm, gvm, asdf) en parallèle d'un Go système (Homebrew, installeur golang.org).

Vérifiez votre environnement :

```bash
which go        # quel binaire est utilisé
go env GOROOT   # doit correspondre à l'installation de ce binaire
```

S'ils diffèrent, désactivez `GOROOT` pour laisser le binaire Go utiliser son propre chemin :

```bash
unset GOROOT
go install github.com/enokdev/helix/cmd/helix@latest
```

Pour corriger définitivement, supprimez toute ligne `export GOROOT=...` dans votre config shell (`~/.zshrc`, `~/.bashrc`) ajoutée par le gestionnaire de versions, ou assurez-vous d'utiliser une seule installation Go à la fois.

:::

## Créer un nouveau projet

```bash
helix new app my-api
cd my-api
```

Cela génère un projet prêt à l'emploi :

```
my-api/
├── main.go
├── go.mod
└── config/
    └── application.yaml
```

Pour ajouter un module (repository, service, contrôleur) :

```bash
helix generate module user
```

## Ajouter à un projet existant

```bash
go get github.com/enokdev/helix@latest
```

## Dépendances du module

Le cœur d'Helix a des dépendances minimales. Les fonctionnalités supplémentaires sont opt-in via les starters :

| Capacité | Dépendance supplémentaire |
|----------|--------------------------|
| Serveur HTTP | `github.com/gofiber/fiber/v2` |
| Base de données SQLite | `gorm.io/driver/sqlite` |
| PostgreSQL | `gorm.io/driver/postgres` |
| Métriques Prometheus | `github.com/prometheus/client_golang` |
| OpenTelemetry | `go.opentelemetry.io/otel` |
| Planification cron | `github.com/robfig/cron/v3` |
| Sécurité JWT | `github.com/golang-jwt/jwt/v5` |

Les starters détectent ces dépendances automatiquement — pas d'enregistrement manuel nécessaire.

## Configuration de l'éditeur

Helix utilise l'outillage Go standard. Tout éditeur avec le support `gopls` fonctionne :

- **VS Code** — [Extension Go](https://marketplace.visualstudio.com/items?itemName=golang.go)
- **GoLand** — support intégré
- **Neovim** — via `nvim-lspconfig` + `gopls`

## Prochaines étapes

- [Démarrage rapide](/fr/guide/quick-start) — construire et exécuter votre première API
