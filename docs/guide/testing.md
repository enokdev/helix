# Testing

Helix includes first-class testing utilities that spin up a real DI container — no mocks-of-mocks, no wiring boilerplate.

## TestApp

`helix.NewTestApp` creates a fully wired container for tests:

```go
import (
    "testing"
    helix "github.com/enokdev/helix"
)

func TestUserService(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &UserRepository{},
            &UserService{},
        ),
        helix.TestConfigPaths("testdata/config"),
    )

    svc := helix.GetBean[*UserService](app)
    user := svc.Create("Alice", "alice@example.com")

    if user.ID == 0 {
        t.Fatal("expected non-zero ID")
    }
}
```

The test app is cleaned up automatically when the test ends.

## Options

### `helix.TestComponents`

Register pre-instantiated components:

```go
helix.TestComponents(
    &UserRepository{},
    &UserService{},
    &EmailService{},
)
```

### `helix.TestConfigPaths`

Load config from custom directories:

```go
helix.TestConfigPaths("testdata/config", "../../config")
```

### `helix.TestConfigDefaults`

Provide in-code config values without a file:

```go
helix.TestConfigDefaults(map[string]any{
    "server.port":       "8181",
    "database.url":      ":memory:",
    "security.jwt.secret": "test-secret",
})
```

### `helix.TestContainerOptions`

Pass raw container options:

```go
helix.TestContainerOptions(
    core.WithShutdownTimeout(5 * time.Second),
)
```

### `helix.MockBean`

Replace a component with a test double:

```go
type MockEmailService struct{}

func (m *MockEmailService) Send(to, subject, body string) error {
    return nil // no-op in tests
}

app := helix.NewTestApp(t,
    helix.TestComponents(&UserService{}),
    helix.MockBean[*EmailService](&MockEmailService{}),
)
```

The mock satisfies the same type as the real component and is injected wherever `*EmailService` is requested.

## Getting Components

```go
svc := helix.GetBean[*UserService](app)
repo := helix.GetBean[*UserRepository](app)
```

`GetBean` panics if the component is not found — appropriate for tests where missing wiring is always a bug.

## HTTP Controller Tests

Test controllers end-to-end without starting a real server:

```go
func TestUserController(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &UserRepository{},
            &UserService{},
            &UserController{},
        ),
    )

    server := helix.GetBean[web.HTTPServer](app)

    // Make an in-process HTTP request
    req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Bob","email":"bob@example.com"}`))
    req.Header.Set("Content-Type", "application/json")

    resp, err := server.ServeHTTP(req)
    if err != nil {
        t.Fatal(err)
    }

    if resp.StatusCode != 201 {
        t.Fatalf("expected 201, got %d", resp.StatusCode)
    }
}
```

`server.ServeHTTP` executes the full handler chain (binding, guards, interceptors) without a network listener.

## Service Tests

```go
func TestOrderService_PlaceOrder(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &ProductRepository{},
            &OrderRepository{},
            &OrderService{},
        ),
        helix.TestConfigDefaults(map[string]any{
            "database.url": ":memory:",
        }),
    )

    svc := helix.GetBean[*OrderService](app)
    order, err := svc.PlaceOrder(ctx, PlaceOrderInput{
        UserID:    "user-1",
        ProductID: 42,
        Quantity:  3,
    })

    if err != nil {
        t.Fatal(err)
    }
    if order.Total <= 0 {
        t.Fatal("expected positive total")
    }
}
```

## Test Data & Config Files

Place test config in a `testdata/` directory:

```
testdata/
└── config/
    └── application.yaml
```

```yaml
# testdata/config/application.yaml
server:
  port: 8181

database:
  url: ":memory:"

security:
  jwt:
    secret: "test-only-secret"
    expiry: "1h"
```

## Integration Tests

For full integration tests that hit a real database:

```go
func TestUserCRUD(t *testing.T) {
    db, err := datagorm.OpenSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    db.AutoMigrate(&User{})

    gormDB := db.Components()[0].(*gorm.DB)
    repo := datagorm.NewRepository[User, uint](gormDB)

    app := helix.NewTestApp(t,
        helix.TestComponents(repo, &UserService{}),
    )

    svc := helix.GetBean[*UserService](app)

    // Create
    user, err := svc.Create("Alice", "alice@example.com")
    require.NoError(t, err)
    require.NotZero(t, user.ID)

    // Read
    found, err := svc.Get(user.ID)
    require.NoError(t, err)
    require.Equal(t, "Alice", found.Name)

    // Update
    updated, err := svc.Update(user.ID, "Alicia", "alicia@example.com")
    require.NoError(t, err)
    require.Equal(t, "Alicia", updated.Name)

    // Delete
    require.NoError(t, svc.Delete(user.ID))
    _, err = svc.Get(user.ID)
    require.ErrorIs(t, err, data.ErrRecordNotFound)
}
```

## Table-Driven Tests

```go
func TestCreateUser_Validation(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(&UserService{}, &UserRepository{}),
    )
    svc := helix.GetBean[*UserService](app)

    cases := []struct {
        name  string
        input CreateUserInput
        want  error
    }{
        {"missing name",  CreateUserInput{Email: "a@b.com"},          ErrValidation},
        {"invalid email", CreateUserInput{Name: "Bob", Email: "bad"}, ErrValidation},
        {"valid",         CreateUserInput{Name: "Bob", Email: "b@c.com"}, nil},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := svc.Create(tc.input)
            if !errors.Is(err, tc.want) {
                t.Errorf("got %v, want %v", err, tc.want)
            }
        })
    }
}
```

## Tips

- Use `TestConfigDefaults` instead of files for simple cases — faster and more portable
- Use `MockBean` for slow external dependencies (email, SMS, S3) but prefer real implementations for repositories
- `server.ServeHTTP` covers the full middleware stack — prefer it over calling service methods directly for controller tests
- In-memory SQLite (`:memory:`) gives you a fresh database per test with no cleanup overhead
