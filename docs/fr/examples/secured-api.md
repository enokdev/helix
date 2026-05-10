# Exemple : API sécurisée (JWT + RBAC)

Une API REST complète avec authentification JWT et contrôle d'accès basé sur les rôles — flux de connexion, endpoints protégés et routes admin uniquement.

## Architecture

```
POST /auth/login     → public — retourne un JWT
GET  /api/profile    → nécessite un JWT valide
GET  /admin/users    → nécessite JWT + rôle admin
```

## Structure du projet

```
secured-api/
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
  name: helix-secured-api

security:
  jwt:
    secret: "super-secret-changez-en-production"
    expiry: "1h"
```

## Code source complet

```go
// main.go
package main

import (
    "log"
    "time"

    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/config"
    "github.com/enokdev/helix/security"
    starter "github.com/enokdev/helix/starter"
    secstarter "github.com/enokdev/helix/starter/security"
    webstarter "github.com/enokdev/helix/starter/web"
    "github.com/enokdev/helix/web"
)

// ── Modèles de domaine ────────────────────────────────────────────────────────

type DemoAccount struct {
    Username string
    Password string
    Roles    []string
}

type AccountInfo struct {
    Username string   `json:"username"`
    Roles    []string `json:"roles"`
}

type UserList struct {
    Users []AccountInfo `json:"users"`
}

type LoginRequest struct {
    Username string `json:"username" validate:"required"`
    Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
    Token string `json:"token"`
}

// ── Service d'authentification ────────────────────────────────────────────────

var demoAccounts = map[string]DemoAccount{
    "user":  {Username: "user",  Password: "password", Roles: []string{"user"}},
    "admin": {Username: "admin", Password: "password", Roles: []string{"user", "admin"}},
}

type AuthService struct {
    helix.Service
    JWTSvc *security.JWTService `inject:"true"`
    expiry time.Duration
}

func (s *AuthService) Authenticate(username, password string) (map[string]any, error) {
    acc, ok := demoAccounts[username]
    if !ok || acc.Password != password {
        return nil, web.Unauthorized("identifiants invalides")
    }
    return map[string]any{
        "sub":   username,
        "roles": acc.Roles,
    }, nil
}

func (s *AuthService) GetInfo(username string) (AccountInfo, bool) {
    acc, ok := demoAccounts[username]
    if !ok {
        return AccountInfo{}, false
    }
    return AccountInfo{Username: acc.Username, Roles: acc.Roles}, true
}

func (s *AuthService) ListAll() []AccountInfo {
    out := make([]AccountInfo, 0, len(demoAccounts))
    for _, acc := range demoAccounts {
        out = append(out, AccountInfo{Username: acc.Username, Roles: acc.Roles})
    }
    return out
}

// ── Contrôleur d'auth (public) ────────────────────────────────────────────────

type AuthController struct {
    helix.Controller
    AuthSvc *AuthService `inject:"true"`
}

//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context, req LoginRequest) (LoginResponse, error) {
    claims, err := c.AuthSvc.Authenticate(req.Username, req.Password)
    if err != nil {
        return LoginResponse{}, err
    }

    token, err := c.AuthSvc.JWTSvc.Generate(claims)
    if err != nil {
        return LoginResponse{}, err
    }

    return LoginResponse{Token: token}, nil
}

// ── Contrôleur API (authentifié) ──────────────────────────────────────────────

type APIController struct {
    helix.Controller
    AuthSvc *AuthService `inject:"true"`
}

//helix:route GET /api/profile
//helix:guard auth
func (c *APIController) Profile(ctx web.Context) (AccountInfo, error) {
    claims := ctx.Locals("claims").(map[string]any)
    username := claims["sub"].(string)

    info, ok := c.AuthSvc.GetInfo(username)
    if !ok {
        return AccountInfo{}, web.NewRequestError(404, "compte introuvable")
    }
    return info, nil
}

// ── Contrôleur Admin (rôle:admin requis) ──────────────────────────────────────

type AdminController struct {
    helix.Controller
    AuthSvc *AuthService `inject:"true"`
}

//helix:route GET /admin/users
//helix:guard role:admin
func (c *AdminController) Users(_ web.Context) (UserList, error) {
    return UserList{Users: c.AuthSvc.ListAll()}, nil
}

// ── Configuration de sécurité ─────────────────────────────────────────────────

type SecurityConfig struct {
    helix.SecurityConfigurer
    JWTSvc *security.JWTService `inject:"true"`
}

func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.Route("/auth/**").PermitAll().
       Route("/api/**").Authenticated().
       Route("/admin/**").Authenticated()
}

// ── Bootstrap ─────────────────────────────────────────────────────────────────

func main() {
    loader := config.NewLoader(
        config.WithConfigPaths("config"),
    )

    if err := helix.Run(helix.App{
        Starters: []starter.Entry{
            {Name: "web",      Order: starter.OrderWeb,      Starter: webstarter.New(loader)},
            {Name: "security", Order: starter.OrderSecurity, Starter: secstarter.New(loader)},
        },
        Components: []any{
            &AuthService{},
            &AuthController{},
            &APIController{},
            &AdminController{},
            &SecurityConfig{},
        },
    }); err != nil {
        log.Fatal(err)
    }
}
```

## Exécution

```bash
go run main.go
# Helix ready on :8080
```

## Démonstration de l'API

### 1. Connexion en tant qu'utilisateur standard

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"password"}'
```

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 2. Accéder au profil protégé

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

curl http://localhost:8080/api/profile \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "username": "user",
  "roles": ["user"]
}
```

### 3. Accéder à l'endpoint admin en tant qu'utilisateur standard

```bash
curl http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "error": {
    "type": "Forbidden",
    "message": "insufficient permissions"
  }
}
```
HTTP 403

### 4. Se connecter en tant qu'admin et accéder à l'endpoint admin

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

ADMIN_TOKEN="..."

curl http://localhost:8080/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

```json
{
  "users": [
    {"username": "user",  "roles": ["user"]},
    {"username": "admin", "roles": ["user", "admin"]}
  ]
}
```

### 5. Accéder sans token

```bash
curl http://localhost:8080/api/profile
```

```json
{
  "error": {
    "type": "Unauthorized",
    "message": "missing or invalid token"
  }
}
```
HTTP 401

### 6. Erreur de validation

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{}'
```

```json
{
  "errors": [
    {"type": "ValidationError", "field": "Username", "message": "required"},
    {"type": "ValidationError", "field": "Password", "message": "required"}
  ]
}
```

## Tests

```go
func TestSecuredAPI(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &AuthService{},
            &AuthController{},
            &APIController{},
            &AdminController{},
            &SecurityConfig{},
        ),
        helix.TestConfigDefaults(map[string]any{
            "security.jwt.secret": "secret-test",
            "security.jwt.expiry": "1h",
        }),
    )

    server := helix.GetBean[web.HTTPServer](app)
    jwtSvc := helix.GetBean[*security.JWTService](app)

    t.Run("connexion réussie", func(t *testing.T) {
        resp, _ := server.ServeHTTP(post("/auth/login",
            `{"username":"user","password":"password"}`))
        require.Equal(t, 200, resp.StatusCode)
    })

    t.Run("profil avec token valide", func(t *testing.T) {
        token, _ := jwtSvc.Generate(map[string]any{
            "sub":   "user",
            "roles": []string{"user"},
        })
        req := get("/api/profile")
        req.Header.Set("Authorization", "Bearer "+token)
        resp, _ := server.ServeHTTP(req)
        require.Equal(t, 200, resp.StatusCode)
    })

    t.Run("endpoint admin nécessite le rôle admin", func(t *testing.T) {
        token, _ := jwtSvc.Generate(map[string]any{
            "sub":   "user",
            "roles": []string{"user"}, // pas de rôle admin
        })
        req := get("/admin/users")
        req.Header.Set("Authorization", "Bearer "+token)
        resp, _ := server.ServeHTTP(req)
        require.Equal(t, 403, resp.StatusCode)
    })

    t.Run("endpoint admin réussit avec token admin", func(t *testing.T) {
        token, _ := jwtSvc.Generate(map[string]any{
            "sub":   "admin",
            "roles": []string{"user", "admin"},
        })
        req := get("/admin/users")
        req.Header.Set("Authorization", "Bearer "+token)
        resp, _ := server.ServeHTTP(req)
        require.Equal(t, 200, resp.StatusCode)
    })

    t.Run("non autorisé sans token", func(t *testing.T) {
        resp, _ := server.ServeHTTP(get("/api/profile"))
        require.Equal(t, 401, resp.StatusCode)
    })
}
```
