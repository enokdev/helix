# Security

Helix provides JWT authentication and role-based access control (RBAC) out of the box, with a declarative configuration API inspired by Spring Security.

## JWT Service

```go
import "github.com/enokdev/helix/security"

jwtSvc, err := security.NewJWTService(
    []byte("your-secret-key"),
    24*time.Hour,  // token expiry
)
```

### Generate a token

```go
token, err := jwtSvc.Generate(map[string]any{
    "sub":   "user-123",
    "email": "alice@example.com",
    "roles": []string{"user", "admin"},
})
```

### Validate a token

```go
claims, err := jwtSvc.Validate(token)
// claims["sub"]   → "user-123"
// claims["roles"] → []string{"user", "admin"}
```

### Refresh a token

```go
newToken, err := jwtSvc.Refresh(expiredToken)
```

Refresh validates the existing token (ignoring expiry) and issues a new one with the same claims and a fresh expiry.

## SecurityConfigurer

Declare which routes require authentication using the `SecurityConfigurer` interface:

```go
type SecurityConfig struct {
    helix.SecurityConfigurer
    JWTSvc *security.JWTService `inject:"true"`
}

func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.Route("/auth/**").PermitAll().      // public endpoints
       Route("/api/**").Authenticated().   // require valid JWT
       Route("/admin/**").Authenticated()  // require valid JWT
}
```

Register your configurer as a component:

```go
helix.Run(helix.App{
    Components: []any{
        &SecurityConfig{},
        // ... other components
    },
})
```

The security starter detects `SecurityConfigurer` in the container and applies the configuration automatically.

## Route Guards

### `//helix:guard auth`

Requires a valid JWT Bearer token. The decoded claims are stored in `ctx.Locals("claims")`.

```go
type APIController struct {
    helix.Controller
    UserSvc *UserService `inject:"true"`
}

//helix:guard auth
func (c *APIController) Profile(ctx web.Context) (UserProfile, error) {
    claims := ctx.Locals("claims").(map[string]any)
    userID := claims["sub"].(string)
    return c.UserSvc.GetProfile(userID)
}
```

### `//helix:guard role:ROLE`

Requires a valid JWT **and** the specified role in the `roles` claim.

```go
type AdminController struct {
    helix.Controller
    AdminSvc *AdminService `inject:"true"`
}

//helix:guard role:admin
func (c *AdminController) Users(_ web.Context) ([]User, error) {
    return c.AdminSvc.ListAll()
}

//helix:guard role:superadmin
func (c *AdminController) DeleteUser(ctx web.Context) error {
    id := ctx.Param("id")
    return c.AdminSvc.Delete(id)
}
```

### Applying a global guard

Protect all routes with a single guard:

```go
httpSecurity := security.NewHTTPSecurity(jwtSvc)
securityConfig.Configure(httpSecurity)
globalGuard := httpSecurity.Build()

web.ApplyGlobalGuard(server, globalGuard)
```

## Full Authentication Flow

Here is a complete login → protected endpoint flow:

```go
// 1. Auth controller — public endpoint
type AuthController struct {
    helix.Controller
    JWTSvc  *security.JWTService `inject:"true"`
    UserSvc *UserService          `inject:"true"`
}

type LoginRequest struct {
    Username string `json:"username" validate:"required"`
    Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
    Token string `json:"token"`
}

//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context, req LoginRequest) (LoginResponse, error) {
    user, err := c.UserSvc.Authenticate(req.Username, req.Password)
    if err != nil {
        return LoginResponse{}, web.Unauthorized("invalid credentials")
    }

    token, err := c.JWTSvc.Generate(map[string]any{
        "sub":   user.ID,
        "email": user.Email,
        "roles": user.Roles,
    })
    if err != nil {
        return LoginResponse{}, err
    }

    return LoginResponse{Token: token}, nil
}

// 2. Protected controller — requires JWT
type ProfileController struct {
    helix.Controller
    UserSvc *UserService `inject:"true"`
}

//helix:guard auth
func (c *ProfileController) Show(ctx web.Context) (UserProfile, error) {
    claims := ctx.Locals("claims").(map[string]any)
    return c.UserSvc.GetProfile(claims["sub"].(string))
}

// 3. Admin controller — requires JWT + admin role
type AdminController struct {
    helix.Controller
    AdminSvc *AdminService `inject:"true"`
}

//helix:guard role:admin
//helix:route GET /admin/users
func (c *AdminController) AllUsers(_ web.Context) ([]User, error) {
    return c.AdminSvc.ListAll()
}
```

## Configuration

Configure JWT via `config/application.yaml`:

```yaml
security:
  jwt:
    secret: "change-me-in-production-use-a-long-random-string"
    expiry: "24h"
```

The security starter reads these values and creates the `JWTService` automatically.

::: warning
Never commit real JWT secrets to version control. Use environment variables in production:
```bash
SECURITY_JWT_SECRET=your-secret go run main.go
```
:::

## Custom Guards

Implement `web.Guard` for custom authorization logic:

```go
type IPAllowlistGuard struct {
    allowed []string
}

func (g *IPAllowlistGuard) CanActivate(ctx web.Context) error {
    for _, ip := range g.allowed {
        if ctx.IP() == ip {
            return nil
        }
    }
    return web.Forbidden("IP not in allowlist")
}

web.RegisterGuard(server, "ip-allowlist", &IPAllowlistGuard{
    allowed: []string{"10.0.0.1", "10.0.0.2"},
})
```

```go
//helix:guard ip-allowlist
func (c *InternalController) Metrics(_ web.Context) (MetricsDump, error) { ... }
```

## Guard Factories

Create guards with arguments using a factory:

```go
web.RegisterGuardFactory(server, "scope", func(argument string) (web.Guard, error) {
    // argument is the part after the colon, e.g. "read" in //helix:guard scope:read
    return &OAuthScopeGuard{required: argument}, nil
})
```

```go
//helix:guard scope:read
func (c *APIController) List(_ web.Context) ([]Resource, error) { ... }

//helix:guard scope:write
func (c *APIController) Create(ctx web.Context, in Input) (Resource, error) { ... }
```

## Accessing the current user

After a `//helix:guard auth` or `//helix:guard role:xxx` validates the request, use helpers to extract the authenticated user:

```go
import "github.com/enokdev/helix/security"

//helix:guard auth
func (c *ProfileController) Show(ctx web.Context) (*UserProfile, error) {
    // Extract the full user struct:
    user, err := security.GetUserFromContext(ctx)
    if err != nil {
        return nil, web.Unauthorized("missing user context")
    }
    // user.ID, user.Email, user.Roles available

    return c.Svc.GetProfile(user.ID)
}

//helix:guard role:admin
func (c *AdminController) Users(ctx web.Context) ([]User, error) {
    // Only need roles:
    roles, err := security.GetRolesFromContext(ctx)
    if err != nil {
        return nil, web.Forbidden("missing role context")
    }
    slog.Info("admin action", "roles", roles)
    return c.Svc.ListAll()
}
```

If you prefer raw claims (e.g., for custom claim fields):

```go
claims := ctx.Locals("claims").(map[string]any)
userID := claims["sub"].(string)
email  := claims["email"].(string)
roles  := claims["roles"].([]string)
```

## Multi-role guards (`HasAnyRole`)

Require one of several roles (OR logic) using the `SecurityConfigurer` API:

```go
func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.
        Route("/auth/**").PermitAll().
        Route("/api/**").Authenticated().
        Route("/admin/**").HasRole("admin").
        Route("/reports/**").HasAnyRole("admin", "analyst", "manager")
}
```

Or with a custom guard factory for inline use:

```go
//helix:guard role:admin,analyst
func (c *ReportController) Index(_ web.Context) ([]Report, error) { ... }
```

## Full `SecurityConfigurer` API

```go
func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.
        // Public endpoints — no auth required
        Route("/auth/**").PermitAll().
        Route("/health").PermitAll().
        Route("/actuator/**").PermitAll().

        // Require valid JWT (any authenticated user)
        Route("/api/**").Authenticated().

        // Require specific role
        Route("/admin/**").HasRole("admin").

        // Require any of the listed roles
        Route("/reports/**").HasAnyRole("admin", "analyst").

        // Deny all (useful as a catch-all at the end)
        Route("/**").Authenticated()
}
```

Rules are evaluated in declaration order — the first matching route wins.

## CORS with security

Configure CORS via a Fiber middleware registered before the guards:

```go
import "github.com/gofiber/fiber/v2/middleware/cors"

server := web.NewServer()

// Register CORS first so preflight OPTIONS requests pass through:
server.Use(cors.New(cors.Config{
    AllowOrigins:     "https://app.example.com",
    AllowHeaders:     "Origin, Content-Type, Authorization",
    AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
    AllowCredentials: true,
}))

// Then register guards and routes:
web.ApplyGlobalGuard(server, jwtGuard)
web.RegisterController(server, &APIController{})
```

For development, allow all origins:

```go
cors.New(cors.Config{AllowOrigins: "*"})
```

## Password hashing

Helix does not hash passwords for you — this is intentional. Use `bcrypt` from the standard library:

```go
import "golang.org/x/crypto/bcrypt"

// When creating a user:
hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
if err != nil {
    return nil, err
}
user.PasswordHash = string(hash)

// When authenticating:
err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plainPassword))
if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
    return nil, web.Unauthorized("invalid credentials")
}
```

Never store plain-text passwords. Never log plain-text passwords.

## Error Responses

| Scenario | Status | Body |
|----------|--------|------|
| Missing token | 401 | `{"error":{"type":"Unauthorized","message":"..."}}` |
| Invalid/expired token | 401 | `{"error":{"type":"Unauthorized","message":"..."}}` |
| Insufficient role | 403 | `{"error":{"type":"Forbidden","message":"..."}}` |
| Token refresh (expired) | 401 | `{"error":{"type":"Unauthorized","message":"token expired"}}` |
