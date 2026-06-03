# Démarrage rapide

Construisez une API REST fonctionnelle en moins de 5 minutes.

## 1. Créer le projet

<CodeTabs :tabs="[{label: 'CLI', key: 'cli'}, {label: 'Manuel', key: 'manual'}]">
  <template #cli>

<TerminalWindow>
$ helix new app my-api<br>
$ cd my-api<br>
$ go mod tidy
</TerminalWindow>

  </template>
  <template #manual>

```bash
mkdir my-api && cd my-api
go mod init my-api
go get github.com/enokdev/helix@latest
```

  </template>
</CodeTabs>

## 2. Définir votre domaine

```go
// internal/user/user.go
package user

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

## 3. Créer un repository

```go
// internal/user/repository.go
package user

import helix "github.com/enokdev/helix"

type Repository struct {
    helix.Repository              // marque ce composant comme repository Helix
    store map[int]User
    next  int
}

func NewRepository() *Repository {
    return &Repository{store: make(map[int]User), next: 1}
}

func (r *Repository) FindAll() []User {
    users := make([]User, 0, len(r.store))
    for _, u := range r.store {
        users = append(users, u)
    }
    return users
}

func (r *Repository) Save(name, email string) User {
    u := User{ID: r.next, Name: name, Email: email}
    r.store[r.next] = u
    r.next++
    return u
}

func (r *Repository) FindByID(id int) (User, bool) {
    u, ok := r.store[id]
    return u, ok
}
```

## 4. Créer un service

```go
// internal/user/service.go
package user

import helix "github.com/enokdev/helix"

type Service struct {
    helix.Service                    // marque ce composant comme service Helix
    Repo *Repository `inject:"true"` // injecté automatiquement
}

func (s *Service) List() []User            { return s.Repo.FindAll() }
func (s *Service) Get(id int) (User, bool) { return s.Repo.FindByID(id) }

type CreateInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func (s *Service) Create(input CreateInput) User {
    return s.Repo.Save(input.Name, input.Email)
}
```

## 5. Créer un contrôleur

```go
// internal/user/controller.go
package user

import (
    "net/http"
    "strconv"

    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
)

type Controller struct {
    helix.Controller               // marque ce composant comme contrôleur Helix
    Svc *Service `inject:"true"`   // injecté automatiquement
}

// GET /users
func (c *Controller) Index() []User {
    return c.Svc.List()
}

// GET /users/:id
func (c *Controller) Show(ctx web.Context) (User, error) {
    id, err := strconv.Atoi(ctx.Param("id"))
    if err != nil {
        return User{}, web.NewRequestError(http.StatusBadRequest, "id invalide")
    }
    u, ok := c.Svc.Get(id)
    if !ok {
        return User{}, web.NewRequestError(http.StatusNotFound, "utilisateur introuvable")
    }
    return u, nil
}

// POST /users
func (c *Controller) Create(ctx web.Context, input CreateInput) (User, error) {
    u := c.Svc.Create(input)
    ctx.Status(http.StatusCreated)
    return u, nil
}
```

## 6. Câbler le tout

```go
// main.go
package main

import (
    "log"

    helix "github.com/enokdev/helix"
    "my-api/internal/user"
)

func main() {
    if err := helix.Run(helix.App{
        Components: []any{
            user.NewRepository(),
            &user.Service{},
            &user.Controller{},
        },
    }); err != nil {
        log.Fatal(err)
    }
}
```

Cet exemple `Components` est manuel pour un package écrit à la main. Si vous créez une fonctionnalité avec `helix generate module` ou `helix generate context`, les `register.go` générés l'enregistrent automatiquement et `helix generate` régénère `helix_imports_gen.go` pour les imports de démarrage.

## 7. Configurer

```yaml
# config/application.yaml
server:
  port: 8080

app:
  name: my-api
```

## 8. Lancer

<TerminalWindow>
$ go run main.go<br>
<span style="color: #27c93f;"># Helix ready on :8080</span>
</TerminalWindow>

Si vous travaillez avec des modules générés, préférez `helix run` pour garder `helix_imports_gen.go` synchronisé automatiquement.

## 9. Tester l'API

<TerminalWindow>
$ curl http://localhost:8080/users<br>
<span style="color: #8b5cf6;">[]</span><br><br>
$ curl -X POST http://localhost:8080/users \<br>
&nbsp;&nbsp;-H "Content-Type: application/json" \<br>
&nbsp;&nbsp;-d '{"name":"Alice","email":"alice@example.com"}'<br>
<span style="color: #8b5cf6;">{"id":1,"name":"Alice","email":"alice@example.com"}</span>
</TerminalWindow>

## Ce qui vient de se passer

- `helix.Controller` intégré dans `Controller` → Helix a enregistré les routes automatiquement
- Les tags `inject:"true"` → Helix a résolu `*Repository` et `*Service` par type
- `validate:"required,email"` → Helix a validé le corps JSON avant d'appeler `Create`
- `ctx.Status(201)` → Helix a retourné 201 Created pour le POST
- La configuration a été chargée depuis `config/application.yaml` automatiquement

## Prochaines étapes

- [Injection de dépendances](/fr/guide/dependency-injection) — comprendre le fonctionnement du conteneur DI
- [Web & Routage](/fr/guide/web) — routes custom, guards, interceptors
- [Exemples](/fr/examples/crud-api) — exemple complet prêt pour la production avec tests
