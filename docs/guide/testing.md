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

## Testing lifecycle hooks

Verify that `OnStart` / `OnStop` behave correctly:

```go
func TestDatabaseConnection_Lifecycle(t *testing.T) {
    db, err := datagorm.OpenSQLite(":memory:")
    if err != nil {
        t.Fatal(err)
    }

    conn := &DatabaseConnection{db: db.DB()}

    // Test OnStart
    if err := conn.OnStart(); err != nil {
        t.Fatalf("OnStart failed: %v", err)
    }

    // Test OnStop
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := conn.OnStop(ctx); err != nil {
        t.Fatalf("OnStop failed: %v", err)
    }
}

func TestScheduler_StartsAndStops(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(&MyJobProvider{}),
    )

    // NewTestApp calls container.Start(), which triggers OnStart on all lifecycle components
    // container.Shutdown() is called automatically on t.Cleanup()

    s := helix.GetBean[scheduler.Scheduler](app)
    entries := s.Entries()
    if len(entries) == 0 {
        t.Fatal("expected at least one scheduled job")
    }
}
```

## HTTP tests with JWT authentication

Test protected endpoints by generating a token in the test:

```go
func TestProfile_Authenticated(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &UserRepository{},
            &UserService{},
            &ProfileController{},
        ),
        helix.TestConfigDefaults(map[string]any{
            "security.jwt.secret": "test-secret-key",
            "security.jwt.expiry": "1h",
        }),
    )

    // Generate a test token:
    jwtSvc := helix.GetBean[*security.JWTService](app)
    token, err := jwtSvc.Generate(map[string]any{
        "sub":   "user-1",
        "email": "alice@example.com",
        "roles": []string{"user"},
    })
    if err != nil {
        t.Fatal(err)
    }

    server := helix.GetBean[web.HTTPServer](app)

    req := httptest.NewRequest("GET", "/profile", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := server.ServeHTTP(req)
    if err != nil {
        t.Fatal(err)
    }
    if resp.StatusCode != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}

func TestProfile_Unauthenticated(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(&ProfileController{}),
        helix.TestConfigDefaults(map[string]any{
            "security.jwt.secret": "test-secret-key",
        }),
    )

    server := helix.GetBean[web.HTTPServer](app)

    req := httptest.NewRequest("GET", "/profile", nil)
    // No Authorization header

    resp, _ := server.ServeHTTP(req)
    if resp.StatusCode != 401 {
        t.Fatalf("expected 401, got %d", resp.StatusCode)
    }
}
```

## Parallel tests

`helix.NewTestApp` is safe to call from parallel tests — each creates its own container:

```go
func TestUserService(t *testing.T) {
    cases := []struct {
        name  string
        email string
        valid bool
    }{
        {"valid", "alice@example.com", true},
        {"invalid email", "not-an-email", false},
        {"empty email", "", false},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()  // each subtest creates its own app

            app := helix.NewTestApp(t,
                helix.TestComponents(&UserService{}),
                helix.TestConfigDefaults(map[string]any{
                    "database.url": ":memory:",
                }),
            )

            svc := helix.GetBean[*UserService](app)
            _, err := svc.Create(tc.email)

            if tc.valid && err != nil {
                t.Fatalf("expected no error, got %v", err)
            }
            if !tc.valid && err == nil {
                t.Fatal("expected error for invalid input")
            }
        })
    }
}
```

## Benchmarks

Use `NewTestApp` in benchmarks to measure realistic performance including DI overhead:

```go
func BenchmarkUserService_Create(b *testing.B) {
    app := helix.NewTestApp(b,
        helix.TestComponents(
            &UserRepository{},
            &UserService{},
        ),
        helix.TestConfigDefaults(map[string]any{
            "database.url": ":memory:",
        }),
    )

    svc := helix.GetBean[*UserService](app)

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            i++
            _, _ = svc.Create(CreateInput{
                Name:  fmt.Sprintf("user-%d", i),
                Email: fmt.Sprintf("user%d@example.com", i),
            })
        }
    })
}
```

## Tips

- Use `TestConfigDefaults` instead of files for simple cases — faster and more portable
- Use `MockBean` for slow external dependencies (email, SMS, S3) but prefer real implementations for repositories
- `server.ServeHTTP` covers the full middleware stack — prefer it over calling service methods directly for controller tests
- In-memory SQLite (`:memory:`) gives you a fresh database per test with no cleanup overhead
- `t.Parallel()` is safe — each `NewTestApp` call creates an isolated container
- Generate JWT tokens in tests using the real `JWTService` — this tests the auth flow end to end
