# Example: CRUD API

A complete, tested REST API for managing users — demonstrating DI, convention routing, request validation, and error handling.

## Project Structure

```
crud-api/
├── main.go
├── go.mod
└── config/
    └── application.yaml
```

## Configuration

```yaml
# config/application.yaml
server:
  port: 8080

app:
  name: helix-crud-api
```

## Full Source

```go
// main.go
package main

import (
    "log"
    "net/http"
    "strconv"
    "sync"

    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
    starter "github.com/enokdev/helix/starter"
    webstarter "github.com/enokdev/helix/starter/web"
    "github.com/enokdev/helix/config"
)

// ── Domain model ────────────────────────────────────────────────────────────

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type userInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

// ── Repository ───────────────────────────────────────────────────────────────

type UserRepository struct {
    helix.Repository
    mu     sync.Mutex
    nextID int
    users  map[int]User
}

func NewUserRepository() *UserRepository {
    return &UserRepository{
        nextID: 1,
        users:  make(map[int]User),
    }
}

func (r *UserRepository) FindAll() []User {
    r.mu.Lock()
    defer r.mu.Unlock()
    out := make([]User, 0, len(r.users))
    for _, u := range r.users {
        out = append(out, u)
    }
    return out
}

func (r *UserRepository) FindByID(id int) (User, bool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    u, ok := r.users[id]
    return u, ok
}

func (r *UserRepository) Save(input userInput) User {
    r.mu.Lock()
    defer r.mu.Unlock()
    u := User{ID: r.nextID, Name: input.Name, Email: input.Email}
    r.users[r.nextID] = u
    r.nextID++
    return u
}

func (r *UserRepository) Update(id int, input userInput) (User, bool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    u, ok := r.users[id]
    if !ok {
        return User{}, false
    }
    u.Name = input.Name
    u.Email = input.Email
    r.users[id] = u
    return u, true
}

func (r *UserRepository) Delete(id int) bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.users[id]; !ok {
        return false
    }
    delete(r.users, id)
    return true
}

// ── Service ───────────────────────────────────────────────────────────────────

type UserService struct {
    helix.Service
    Repo *UserRepository `inject:"true"`
}

func (s *UserService) List() []User                    { return s.Repo.FindAll() }
func (s *UserService) Get(id int) (User, bool)         { return s.Repo.FindByID(id) }
func (s *UserService) Create(in userInput) User        { return s.Repo.Save(in) }
func (s *UserService) Update(id int, in userInput) (User, bool) { return s.Repo.Update(id, in) }
func (s *UserService) Delete(id int) bool              { return s.Repo.Delete(id) }

// ── Controller ────────────────────────────────────────────────────────────────

type UserController struct {
    helix.Controller
    Service *UserService `inject:"true"`
}

// GET /users
func (c *UserController) Index() []User {
    return c.Service.List()
}

// GET /users/:id
func (c *UserController) Show(ctx web.Context) (User, error) {
    id, err := userID(ctx)
    if err != nil {
        return User{}, err
    }
    u, ok := c.Service.Get(id)
    if !ok {
        return User{}, notFound()
    }
    return u, nil
}

// POST /users  →  201 Created
func (c *UserController) Create(ctx web.Context, input userInput) (User, error) {
    u := c.Service.Create(input)
    ctx.Status(http.StatusCreated)
    return u, nil
}

// PUT /users/:id
func (c *UserController) Update(ctx web.Context, input userInput) (User, error) {
    id, err := userID(ctx)
    if err != nil {
        return User{}, err
    }
    u, ok := c.Service.Update(id, input)
    if !ok {
        return User{}, notFound()
    }
    return u, nil
}

// DELETE /users/:id  →  204 No Content
func (c *UserController) Delete(ctx web.Context) error {
    id, err := userID(ctx)
    if err != nil {
        return err
    }
    if !c.Service.Delete(id) {
        return notFound()
    }
    ctx.Status(http.StatusNoContent)
    return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func userID(ctx web.Context) (int, error) {
    id, err := strconv.Atoi(ctx.Param("id"))
    if err != nil || id < 1 {
        return 0, web.NewRequestError(http.StatusBadRequest, "invalid user id")
    }
    return id, nil
}

func notFound() error {
    return web.NewRequestError(http.StatusNotFound, "user not found")
}

// ── Bootstrap ─────────────────────────────────────────────────────────────────

func main() {
    loader := config.NewLoader(
        config.WithConfigPaths("config"),
    )

    if err := helix.Run(helix.App{
        Starters: []starter.Entry{
            {Name: "web", Order: starter.OrderWeb, Starter: webstarter.New(loader)},
        },
        Components: []any{
            NewUserRepository(),
            &UserService{},
            &UserController{},
        },
    }); err != nil {
        log.Fatal(err)
    }
}
```

## Generated Routes

| Method | Path | Handler |
|--------|------|---------|
| `GET` | `/users` | `UserController.Index` |
| `GET` | `/users/:id` | `UserController.Show` |
| `POST` | `/users` | `UserController.Create` |
| `PUT` | `/users/:id` | `UserController.Update` |
| `DELETE` | `/users/:id` | `UserController.Delete` |

## Running

```bash
go run main.go
# Helix ready on :8080
```

## API Walkthrough

### List users

```bash
curl http://localhost:8080/users
# []
```

### Create a user

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
# {"id":1,"name":"Alice","email":"alice@example.com"}
# HTTP 201 Created
```

### Get by ID

```bash
curl http://localhost:8080/users/1
# {"id":1,"name":"Alice","email":"alice@example.com"}
```

### Update

```bash
curl -X PUT http://localhost:8080/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Alicia","email":"alicia@example.com"}'
# {"id":1,"name":"Alicia","email":"alicia@example.com"}
```

### Delete

```bash
curl -X DELETE http://localhost:8080/users/1
# HTTP 204 No Content
```

### Not found

```bash
curl http://localhost:8080/users/999
# HTTP 404
# {"error":{"type":"NotFound","message":"user not found"}}
```

### Validation error

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"","email":"not-an-email"}'
# HTTP 400
# {"errors":[
#   {"type":"ValidationError","field":"Name","message":"required"},
#   {"type":"ValidationError","field":"Email","message":"must be a valid email address"}
# ]}
```

## Tests

```go
// main_test.go
package main

import (
    "net/http"
    "strings"
    "testing"

    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
)

func testApp(t *testing.T) *helix.TestApp {
    t.Helper()
    return helix.NewTestApp(t,
        helix.TestComponents(
            NewUserRepository(),
            &UserService{},
            &UserController{},
        ),
    )
}

func TestUsersCRUD(t *testing.T) {
    app := testApp(t)
    server := helix.GetBean[web.HTTPServer](app)

    // Create
    resp, _ := server.ServeHTTP(newRequest("POST", "/users",
        `{"name":"Alice","email":"alice@example.com"}`))
    if resp.StatusCode != http.StatusCreated {
        t.Fatalf("expected 201, got %d", resp.StatusCode)
    }

    // List
    resp, _ = server.ServeHTTP(newRequest("GET", "/users", ""))
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }

    // Delete
    resp, _ = server.ServeHTTP(newRequest("DELETE", "/users/1", ""))
    if resp.StatusCode != http.StatusNoContent {
        t.Fatalf("expected 204, got %d", resp.StatusCode)
    }

    // Not found
    resp, _ = server.ServeHTTP(newRequest("GET", "/users/1", ""))
    if resp.StatusCode != http.StatusNotFound {
        t.Fatalf("expected 404, got %d", resp.StatusCode)
    }
}

func newRequest(method, path, body string) *http.Request {
    req, _ := http.NewRequest(method, path, strings.NewReader(body))
    if body != "" {
        req.Header.Set("Content-Type", "application/json")
    }
    return req
}
```

```bash
go test ./...
# ok   crud-api  0.012s
```
