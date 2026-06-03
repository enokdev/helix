# Utiliser le CLI

Le CLI Helix est votre outil principal pour créer des projets, générer du code et gérer le cycle de vie du développement. Ce guide présente chaque workflow dans l'ordre où vous les rencontrerez typiquement.

## Installer le CLI

```bash
go install github.com/enokdev/helix/cmd/helix@latest
helix version
```

Le binaire est placé dans `$GOPATH/bin` (généralement `~/go/bin`). Si votre shell ne le trouve pas, ajoutez ce répertoire à votre `PATH`.

---

## Créer un projet

```bash
helix new app my-api
cd my-api
go mod tidy
```

`helix new app` crée un scaffold minimal et fonctionnel :

```
my-api/
├── main.go                  # point d'entrée de l'application
├── go.mod                   # module Go (référence helix à la bonne version)
└── config/
    └── application.yaml     # port serveur, nom de l'app, et clés personnalisées
```

Le `main.go` généré appelle `helix.Run()` — le point d'entrée unique qui démarre le conteneur DI, le serveur HTTP et gère les signaux OS :

```go
func main() {
    if err := helix.Run(helix.App{}); err != nil {
        log.Fatal(err)
    }
}
```

`helix.App` accepte une slice `Components` pour les composants câblés manuellement. Quand vous utilisez des modules ou contextes générés, Helix les enregistre automatiquement.

---

## Ajouter des modules de fonctionnalités

Une fois le projet créé, générez un module de fonctionnalité avec :

```bash
helix generate module order
```

Cela crée quatre fichiers dans un package `orders/` (singulier → pluriel automatiquement) :

```
orders/
├── controller.go   # gestionnaire HTTP câblé au service
├── service.go      # logique métier, reçoit le repository via DI
├── repository.go   # couche d'accès aux données, intègre helix.Repository
└── register.go     # auto-enregistrement via init()
```

Chaque fichier a le bon type intégré et le tag `inject:"true"` déjà en place — vous n'avez qu'à implémenter les méthodes. `register.go` appelle `helix.RegisterComponents(...)`, donc il n'y a plus d'étape d'enregistrement manuel dans `main.go` pour les modules générés.

Helix lit toujours les tags `inject:"true"` et câble `OrderRepository → OrderService → OrderController` automatiquement.

---

## Contextes bornés DDD

Pour les fonctionnalités avec une frontière de domaine claire, utilisez un contexte borné plutôt qu'un module simple :

```bash
helix generate context billing
```

Cela génère un package `billings/` avec cinq fichiers :

```
billings/
├── api.go          # fonctions de domaine pures : CreateBilling(), GetBilling()
├── repository.go   # accès aux données
├── service.go      # logique métier avec stubs de méthodes Create/Get
├── controller.go   # routes HTTP
└── register.go     # auto-enregistrement via init()
```

L'addition clé est `api.go`. Il expose les opérations de domaine comme des fonctions Go pures, découplées de HTTP et de la base de données. D'autres packages de votre application peuvent appeler `billings.CreateBilling(ctx, attrs)` sans rien savoir de la couche HTTP ou du conteneur DI. Comme les modules générés, les contextes bornés reçoivent aussi un `register.go`, donc aucun câblage manuel dans `main.go` n'est nécessaire.

Utilisez un contexte borné quand :
- Une fonctionnalité a son propre langage de domaine (entités, objets de valeur)
- Vous voulez une API interne stable dont d'autres packages peuvent dépendre
- La fonctionnalité pourrait éventuellement devenir son propre service

---

## Régénérer le code

Après avoir modifié ou ajouté des contrôleurs, exécutez :

```bash
helix generate
```

Cela scanne le projet et régénère les fichiers d'enregistrement des routes et de câblage DI, y compris `helix_imports_gen.go` avec des blank imports qui déclenchent les enregistrements `init()` des modules générés. Vous le ferez typiquement après :
- Avoir ajouté une nouvelle méthode à un contrôleur
- Avoir renommé un contrôleur
- Avoir ajouté ou supprimé un guard ou un intercepteur
- Avoir ajouté un nouveau module ou contexte généré

Pour le câblage DI à la compilation (style Wire, sans réflexion à l'exécution), utilisez :

```bash
helix generate wire
```

---

## Développement : rechargement à chaud

Pendant le développement, utilisez `helix run` plutôt que `go run` :

```bash
helix run
```

Il surveille vos fichiers source pour les modifications. Quand vous sauvegardez un fichier `.go`, il recompile et redémarre le processus automatiquement — pas de cycle stop/start manuel. Avant chaque build, il exécute `helix generate`, ce qui régénère `helix_imports_gen.go` pour auto-enregistrer les nouveaux modules générés.

Le processus gère `SIGINT` (`Ctrl+C`) et `SIGTERM` avec grâce : les requêtes actives se terminent, et les hooks de cycle de vie `OnStop()` s'exécutent avant la sortie du processus.

---

## Construire pour la production

```bash
# Compiler un binaire statique
helix build

# Générer un Dockerfile (multi-étapes, image finale minimale)
helix build --docker
```

Le binaire est placé à la racine du projet. Envoyez-le sur n'importe quel serveur Linux ou conteneur et exécutez-le directement — aucun runtime Go requis.

---

## Migrations de base de données

Helix gère les migrations de schéma avec des fichiers SQL simples.

### 1. Créer des fichiers de migration

```bash
helix db migrate create add-orders-table
```

Deux fichiers sont créés dans `migrations/` :

```
migrations/
├── 20240115120000_add-orders-table.up.sql
└── 20240115120000_add-orders-table.down.sql
```

Éditez-les avec votre SQL :

```sql
-- up.sql
CREATE TABLE orders (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    total   REAL NOT NULL
);

-- down.sql
DROP TABLE IF EXISTS orders;
```

### 2. Appliquer les migrations

```bash
helix db migrate up
```

Toutes les migrations en attente sont appliquées en ordre chronologique. Pour cibler une base de données spécifique (en écrasant `application.yaml`) :

```bash
helix db migrate up --database-url postgres://user:pass@localhost/mydb
```

### 3. Vérifier le statut

```bash
helix db migrate status
```

```
Migration                                     Status
----------------------------------------------------
20240115120000_create-users-table             applied
20240116090000_add-orders-table               pending
```

### 4. Annuler

```bash
helix db migrate down
```

Annule une migration à la fois (la plus récemment appliquée).

---

## Code généré

`helix generate` crée deux types de fichiers :

### Enregistrement des routes (`_helix_routes_gen.go`)

```go
// Code généré par helix generate. NE PAS MODIFIER.
package main

import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
    "my-api/orders"
)

func init() {
    helix.RegisterWebSetup(func() error {
        // Les routes sont enregistrées ici depuis les directives du contrôleur
        return nil
    })
}
```

### Câblage DI Wire (`wire_gen.go`)

`helix generate wire` génère le câblage DI à la compilation :

```go
// Code généré par helix generate wire. NE PAS MODIFIER.
package main

import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/core"
    "my-api/orders"
    "my-api/user"
)

func init() {
    helix.RegisterWireSetup(func(c *core.Container) error {
        userRepo := &user.Repository{}
        userSvc := &user.Service{Repo: userRepo}
        userCtrl := &user.Controller{Svc: userSvc}

        c.Register(userRepo)
        c.Register(userSvc)
        c.Register(userCtrl)

        orderRepo := &orders.OrderRepository{}
        orderSvc := &orders.OrderService{Repo: orderRepo, UserSvc: userSvc}
        orderCtrl := &orders.OrderController{Svc: orderSvc}

        c.Register(orderRepo)
        c.Register(orderSvc)
        c.Register(orderCtrl)
        return nil
    })
}
```

Avec le mode Wire, aucune réflexion ne se produit à l'exécution. Utilisez-le quand :
- Le temps de démarrage est critique
- Vous voulez une vérification à la compilation que toutes les dépendances sont satisfaites
- Vous déployez dans des environnements avec des ressources limitées

## Flags de build

Passez des flags de build Go via `helix build` :

```bash
# Embarquer les informations de version :
helix build --ldflags="-X main.version=$(git describe --tags) -X main.commit=$(git rev-parse --short HEAD)"

# Compilation croisée pour Linux :
GOOS=linux GOARCH=amd64 helix build

# Désactiver CGO pour un binaire entièrement statique (requis pour les images Docker distroless/scratch) :
CGO_ENABLED=0 helix build

# Tout combiné :
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 helix build \
  --ldflags="-s -w -X main.version=1.2.3"
```

## Dockerfile (généré par `helix build --docker`)

```dockerfile
# --- Étape de build ---
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# --- Étape d'exécution ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ ./config/
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

L'image de base distroless n'a pas de shell, pas de gestionnaire de paquets, pas de `apt` — une surface d'attaque significativement réduite par rapport à `alpine`.

## Intégration CI/CD

Ajoutez la génération de code et le build à votre pipeline :

```yaml
# .github/workflows/ci.yml
- name: Générer
  run: helix generate

- name: Build
  run: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 helix build

- name: Test
  run: go test ./...
```

Si `helix generate` produit une sortie qui diffère de ce qui est commité, le pipeline échoue — cela détecte les fichiers générés périmés :

```yaml
- name: Vérifier que les fichiers générés sont à jour
  run: |
    helix generate
    git diff --exit-code   # échoue si les fichiers générés ont changé
```

## Référence complète des commandes

Voir la [Référence CLI](/fr/reference/cli) pour la liste complète de tous les flags et options.
