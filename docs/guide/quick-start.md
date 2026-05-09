# Quick Start

Build a working REST API in under 5 minutes.

## 1. Créer le projet

<CodeTabs :tabs="[{label: 'CLI', key: 'cli'}, {label: 'Manual', key: 'manual'}]">
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

## 2. Define your domain

```go
// internal/user/user.go
package user

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

## 3. Create a repository

```go
// internal/user/repository.go
package user

import helix "github.com/enokdev/helix"

type Repository struct {
    helix.Repository              // marks this as a Helix repository component
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

## 4. Create a service

```go
// internal/user/service.go
package user

import helix "github.com/enokdev/helix"

type Service struct {
    helix.Service                 // marks this as a Helix service component
    Repo *Repository `inject:"true"` // injected automatically
}

func (s *Service) List() []User           { return s.Repo.FindAll() }
func (s *Service) Get(id int) (User, bool) { return s.Repo.FindByID(id) }

type CreateInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func (s *Service) Create(input CreateInput) User {
    return s.Repo.Save(input.Name, input.Email)
}
```

## 5. Create a controller

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
    helix.Controller              // marks this as a Helix controller
    Svc *Service `inject:"true"`  // injected automatically
}

// GET /users
func (c *Controller) Index() []User {
    return c.Svc.List()
}

// GET /users/:id
func (c *Controller) Show(ctx web.Context) (User, error) {
    id, err := strconv.Atoi(ctx.Param("id"))
    if err != nil {
        return User{}, web.NewRequestError(http.StatusBadRequest, "invalid id")
    }
    u, ok := c.Svc.Get(id)
    if !ok {
        return User{}, web.NewRequestError(http.StatusNotFound, "user not found")
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

## 6. Wire it up

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

## 7. Configure

```yaml
# config/application.yaml
server:
  port: 8080

app:
  name: my-api
```

## 8. Run

<TerminalWindow>
$ go run main.go<br>
<span style="color: #27c93f;"># Helix ready on :8080</span>
</TerminalWindow>

## 9. Test the API

<TerminalWindow>
$ curl http://localhost:8080/users<br>
<span style="color: #8b5cf6;">[]</span><br><br>
$ curl -X POST http://localhost:8080/users \<br>
&nbsp;&nbsp;-H "Content-Type: application/json" \<br>
&nbsp;&nbsp;-d '{"name":"Alice","email":"alice@example.com"}'<br>
<span style="color: #8b5cf6;">{"id":1,"name":"Alice","email":"alice@example.com"}</span>
</TerminalWindow>

## What just happened?

- `helix.Controller` embedded in `Controller` → Helix registered routes automatically
- `inject:"true"` tags → Helix resolved `*Repository` and `*Service` by type
- `validate:"required,email"` → Helix validated the JSON body before calling `Create`
- `ctx.Status(201)` → Helix returned 201 Created for POST
- Configuration loaded from `config/application.yaml` automatically

## Next Steps

- [Dependency Injection](/guide/dependency-injection) — understand how the DI container works
- [Web & Routing](/guide/web) — custom routes, guards, interceptors
- [Examples](/examples/crud-api) — full production-ready example with tests
