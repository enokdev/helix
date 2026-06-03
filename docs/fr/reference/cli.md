# Référence CLI

Le CLI Helix permet de créer des projets, générer du code, gérer les migrations de base de données et exécuter votre application pendant le développement.

## Installation

```bash
go install github.com/enokdev/helix/cmd/helix@latest
```

Vérifier :

```bash
helix version
```

::: details `helix: command not found` après installation
`go install` place le binaire dans `$GOPATH/bin` (par défaut `~/go/bin`). Ajoutez-le à votre `PATH` :

```bash
export PATH="$PATH:$HOME/go/bin"
source ~/.zshrc   # ou ~/.bashrc
```
:::

---

## Session de développement typique

```bash
# 1. Créer un nouveau projet
helix new app my-api
cd my-api

# 2. Récupérer les dépendances
go mod tidy

# 3. Générer un module de fonctionnalité
helix generate module order

# 4. Câbler les composants, écrire la logique métier…

# 5. Démarrer avec rechargement à chaud
helix run

# 6. Construire un binaire de production
helix build
```

---

## `helix version`

Afficher la version du CLI installée.

```bash
helix version
# ou
helix --version
```

**Sortie :**

```
helix v1.1.2
```

---

## `helix new app`

Créer un squelette d'application Helix prête à l'emploi.

```bash
helix new app <nom> [flags]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom` | Nom du projet — devient le nom du répertoire et du module Go |

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Répertoire parent dans lequel créer le dossier de l'app |

**Exemples :**

```bash
helix new app my-api
helix new app my-api --dir /workspace
```

**Structure générée :**

```
my-api/
├── main.go
├── go.mod
└── config/
    └── application.yaml
```

**`main.go`**

```go
package main

import (
    "log"

    "github.com/enokdev/helix"
)

func main() {
    if err := helix.Run(helix.App{}); err != nil {
        log.Fatal(err)
    }
}
```

**`config/application.yaml`**

```yaml
app:
  name: my-api
server:
  port: 8080
```

Après le scaffold, exécutez `go mod tidy` pour télécharger les dépendances, puis ajoutez des modules de fonctionnalités avec [`helix generate module`](#helix-generate-module).

---

## `helix generate module`

Ajouter un module de fonctionnalité (contrôleur + service + repository) à un projet existant.

```bash
helix generate module <nom> [flags]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom` | Nom du module au singulier (ex. `order`, `product`, `user`) |

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go (doit contenir `go.mod`) |

**Exemples :**

```bash
helix generate module order
helix generate module product --dir ./my-api
```

**Structure générée** (`helix generate module order`) :

```
orders/
├── controller.go
├── service.go
├── repository.go
└── register.go
```

Le nom du dossier est automatiquement mis au pluriel (`order` → `orders`). `register.go` appelle `helix.RegisterComponents(...)` depuis `init()`, donc les composants générés sont câblés automatiquement sans enregistrement manuel dans `main.go`.

**`orders/repository.go`**

```go
package orders

import "github.com/enokdev/helix"

type OrderRepository struct {
    helix.Repository
}
```

**`orders/service.go`**

```go
package orders

import "github.com/enokdev/helix"

type OrderService struct {
    helix.Service
    Repository *OrderRepository `inject:"true"`
}
```

**`orders/controller.go`**

```go
package orders

import (
    "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
)

type OrderController struct {
    helix.Controller
    Service *OrderService `inject:"true"`
}

func (c *OrderController) Index(ctx web.Context) error {
    return ctx.JSON(map[string]string{"module": "orders"})
}
```

Après la génération, aucune mise à jour de `main.go` n'est nécessaire. Le fichier `register.go` généré gère l'enregistrement DI automatiquement.

---

## `helix generate context`

Générer un contexte borné de style DDD — un package autonome avec sa propre API de domaine, repository, service et contrôleur.

```bash
helix generate context <nom> [flags]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom` | Nom du contexte (ex. `billing`, `inventory`, `accounts`) |

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go (doit contenir `go.mod`) |

**Exemples :**

```bash
helix generate context billing
helix generate context accounts --dir ./my-api
```

**Structure générée** (`helix generate context billing`) :

```
billings/
├── api.go          # fonctions de domaine publiques (Create, Get)
├── repository.go   # accès aux données
├── service.go      # logique métier
├── controller.go   # couche HTTP
└── register.go     # auto-enregistrement via init()
```

**`billings/api.go`** — le point d'entrée public du contexte, sans dépendances HTTP ou DB :

```go
package billings

import (
    "context"
    "errors"
)

var ErrNotImplemented = errors.New("billings: opération de contexte non implémentée")

type BillingID string

type Billing struct {
    ID BillingID
}

type CreateBillingAttrs struct {
    Name string
}

func CreateBilling(ctx context.Context, attrs CreateBillingAttrs) (*Billing, error) {
    return newBillingService().CreateBilling(ctx, attrs)
}

func GetBilling(ctx context.Context, id BillingID) (*Billing, error) {
    return newBillingService().GetBilling(ctx, id)
}
```

Utilisez un contexte borné quand une fonctionnalité a une frontière de domaine claire et que vous voulez exposer une API Go propre (pas seulement des routes HTTP) au reste de l'application. Comme pour `helix generate module`, le `register.go` généré enregistre automatiquement les composants du contexte, sans câblage manuel dans `main.go`.

---

## `helix generate` (génération de code Wire)

Scanner le projet et régénérer le code de routage et de câblage DI depuis les annotations source.

```bash
helix generate [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Arbre de répertoires à scanner |

**Ce qu'il génère :**

- Enregistrements de routes dérivés des signatures de méthodes des contrôleurs
- Enregistrements de guards et d'intercepteurs
- `helix_imports_gen.go` avec des blank imports vers les packages générés afin que leurs auto-enregistrements `init()` s'exécutent au démarrage

Exécutez ceci après avoir ajouté ou renommé des contrôleurs, guards, intercepteurs, modules ou contextes.

```bash
helix generate
helix generate --dir ./my-api
```

---

## `helix generate wire`

Générer des liaisons d'injection de dépendances à la compilation (style Wire).

```bash
helix generate wire [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Arbre de répertoires à scanner |

**Quand utiliser :** Si vous préférez la DI à la compilation plutôt que le resolver basé sur la réflexion. Après `helix generate wire`, le fichier généré remplace la résolution `inject:"true"` à l'exécution par des appels de constructeurs explicites.

```bash
helix generate wire
```

---

## `helix run`

Démarrer l'application avec rechargement à chaud. Surveille les modifications des fichiers source et redémarre automatiquement.

```bash
helix run [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |

**Exemples :**

```bash
helix run
helix run --dir ./my-api
```

::: tip Développement vs production
`helix run` est uniquement pour le développement — il rebuild et redémarre à chaque sauvegarde. Pour la production, utilisez `helix build` pour produire un binaire statique et exécutez-le directement.
:::

Avant chaque build, `helix run` exécute `helix generate`, ce qui régénère `helix_imports_gen.go` pour que les nouveaux modules générés soient auto-enregistrés au démarrage.

Le processus gère `SIGINT` et `SIGTERM` avec grâce : les requêtes en vol se terminent et les hooks de cycle de vie `OnStop()` s'exécutent avant la sortie.

---

## `helix build`

Compiler l'application en binaire de production (ou générer un Dockerfile).

```bash
helix build [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |
| `--docker` | `false` | Générer un Dockerfile plutôt que de construire un binaire |

**Exemples :**

```bash
# Binaire standard
helix build

# Dockerfile pour déploiement en conteneur
helix build --docker
```

Le binaire est produit à la racine du projet. Le flag Docker produit un `Dockerfile` multi-étapes avec une image finale minimale.

---

## `helix db migrate create`

Créer une paire de fichiers SQL de migration horodatés.

```bash
helix db migrate create <nom> [flags]
```

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom` | Nom descriptif de la migration (utilisez des tirets, ex. `add-orders-table`) |

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |

**Exemple :**

```bash
helix db migrate create add-orders-table
```

**Fichiers générés :**

```
migrations/
├── 20240115120000_add-orders-table.up.sql    # migration avant
└── 20240115120000_add-orders-table.down.sql  # rollback
```

Remplissez le `.up.sql` avec votre changement de schéma, et le `.down.sql` avec le rollback :

```sql
-- 20240115120000_add-orders-table.up.sql
CREATE TABLE orders (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    total   REAL NOT NULL
);

-- 20240115120000_add-orders-table.down.sql
DROP TABLE IF EXISTS orders;
```

---

## `helix db migrate up`

Appliquer toutes les migrations en attente dans l'ordre chronologique.

```bash
helix db migrate up [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |
| `--database-url` | depuis la config | URL de connexion à la base de données (écrase `database.url` dans `application.yaml`) |

**Exemples :**

```bash
helix db migrate up
helix db migrate up --database-url postgres://localhost/mydb
```

---

## `helix db migrate down`

Annuler la migration appliquée la plus récemment.

```bash
helix db migrate down [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |
| `--database-url` | depuis la config | URL de connexion à la base de données |

```bash
helix db migrate down
```

Chaque appel annule exactement une migration. Exécutez plusieurs fois pour reculer davantage.

---

## `helix db migrate status`

Afficher quelles migrations ont été appliquées et lesquelles sont en attente.

```bash
helix db migrate status [flags]
```

**Flags :**

| Flag | Défaut | Description |
|------|--------|-------------|
| `--dir` | `.` | Racine du module Go |
| `--database-url` | depuis la config | URL de connexion à la base de données |

**Exemple de sortie :**

```
Migration                                     Status
----------------------------------------------------
20240115120000_create-users-table             applied
20240116090000_add-orders-table               applied
20240117140000_add-product-index              pending
```

---

## Récapitulatif des commandes

| Commande | But |
|----------|-----|
| `helix version` | Afficher la version du CLI |
| `helix new app <nom>` | Créer un nouveau projet |
| `helix generate module <nom>` | Ajouter un module de fonctionnalité (contrôleur/service/repository) |
| `helix generate context <nom>` | Ajouter un contexte borné DDD |
| `helix generate` | Régénérer le code de routage et de câblage DI |
| `helix generate wire` | Générer les liaisons DI à la compilation |
| `helix run` | Démarrer avec rechargement à chaud (développement) |
| `helix build` | Compiler le binaire de production |
| `helix build --docker` | Générer un Dockerfile |
| `helix db migrate create <nom>` | Créer des fichiers de migration |
| `helix db migrate up` | Appliquer les migrations en attente |
| `helix db migrate down` | Annuler la dernière migration |
| `helix db migrate status` | Afficher le statut des migrations |

---

## Variables d'environnement

| Variable | Description |
|----------|-------------|
| `HELIX_PROFILES_ACTIVE` | Liste de profils de config actifs séparés par des virgules |
| `DATABASE_URL` | Écrase `database.url` dans `application.yaml` |
| `SERVER_PORT` | Écrase `server.port` dans `application.yaml` |
