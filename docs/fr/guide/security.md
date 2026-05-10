# Sécurité

Helix fournit l'authentification JWT et le contrôle d'accès basé sur les rôles (RBAC) nativement, avec une API de configuration déclarative inspirée de Spring Security.

## Service JWT

```go
import "github.com/enokdev/helix/security"

jwtSvc, err := security.NewJWTService(
    []byte("votre-clé-secrète"),
    24*time.Hour,  // expiration du token
)
```

### Générer un token

```go
token, err := jwtSvc.Generate(map[string]any{
    "sub":   "user-123",
    "email": "alice@example.com",
    "roles": []string{"user", "admin"},
})
```

### Valider un token

```go
claims, err := jwtSvc.Validate(token)
// claims["sub"]   → "user-123"
// claims["roles"] → []string{"user", "admin"}
```

### Renouveler un token

```go
newToken, err := jwtSvc.Refresh(expiredToken)
```

Refresh valide le token existant (en ignorant l'expiration) et émet un nouveau avec les mêmes claims et une nouvelle expiration.

## SecurityConfigurer

Déclarez quelles routes nécessitent une authentification via l'interface `SecurityConfigurer` :

```go
type SecurityConfig struct {
    helix.SecurityConfigurer
    JWTSvc *security.JWTService `inject:"true"`
}

func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.Route("/auth/**").PermitAll().      // endpoints publics
       Route("/api/**").Authenticated().   // nécessite un JWT valide
       Route("/admin/**").Authenticated()  // nécessite un JWT valide
}
```

Enregistrez votre configurateur comme composant :

```go
helix.Run(helix.App{
    Components: []any{
        &SecurityConfig{},
        // ... autres composants
    },
})
```

Le starter de sécurité détecte `SecurityConfigurer` dans le conteneur et applique la configuration automatiquement.

### API complète de `SecurityConfigurer`

```go
func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.
        // Endpoints publics — aucune auth requise
        Route("/auth/**").PermitAll().
        Route("/health").PermitAll().
        Route("/actuator/**").PermitAll().

        // Nécessite un JWT valide (tout utilisateur authentifié)
        Route("/api/**").Authenticated().

        // Nécessite un rôle spécifique
        Route("/admin/**").HasRole("admin").

        // Nécessite l'un des rôles listés
        Route("/reports/**").HasAnyRole("admin", "analyst").

        // Refuser tout (utile comme catch-all à la fin)
        Route("/**").Authenticated()
}
```

Les règles sont évaluées dans l'ordre de déclaration — la première route correspondante gagne.

## Guards de route

### `//helix:guard auth`

Nécessite un token JWT Bearer valide. Les claims décodés sont stockés dans `ctx.Locals("claims")`.

```go
//helix:guard auth
func (c *APIController) Profile(ctx web.Context) (UserProfile, error) {
    claims := ctx.Locals("claims").(map[string]any)
    userID := claims["sub"].(string)
    return c.UserSvc.GetProfile(userID)
}
```

### `//helix:guard role:ROLE`

Nécessite un JWT valide **et** le rôle spécifié dans le claim `roles`.

```go
//helix:guard role:admin
func (c *AdminController) Users(_ web.Context) ([]User, error) {
    return c.AdminSvc.ListAll()
}
```

## Accéder à l'utilisateur courant

Après validation par un guard, utilisez les helpers pour extraire l'utilisateur authentifié :

```go
import "github.com/enokdev/helix/security"

//helix:guard auth
func (c *ProfileController) Show(ctx web.Context) (*UserProfile, error) {
    // Extraire l'utilisateur complet :
    user, err := security.GetUserFromContext(ctx)
    if err != nil {
        return nil, web.Unauthorized("contexte utilisateur manquant")
    }
    // user.ID, user.Email, user.Roles disponibles

    return c.Svc.GetProfile(user.ID)
}

//helix:guard role:admin
func (c *AdminController) Users(ctx web.Context) ([]User, error) {
    // Uniquement les rôles :
    roles, err := security.GetRolesFromContext(ctx)
    if err != nil {
        return nil, web.Forbidden("contexte de rôle manquant")
    }
    slog.Info("action admin", "rôles", roles)
    return c.Svc.ListAll()
}
```

Si vous préférez les claims bruts (ex. pour des champs de claims personnalisés) :

```go
claims := ctx.Locals("claims").(map[string]any)
userID := claims["sub"].(string)
email  := claims["email"].(string)
roles  := claims["roles"].([]string)
```

## Guard multi-rôle (`HasAnyRole`)

Requérir l'un de plusieurs rôles (logique OR) via l'API `SecurityConfigurer` :

```go
func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
    hs.
        Route("/auth/**").PermitAll().
        Route("/api/**").Authenticated().
        Route("/admin/**").HasRole("admin").
        Route("/reports/**").HasAnyRole("admin", "analyst", "manager")
}
```

## Flux d'authentification complet

```go
// 1. Contrôleur d'auth — endpoint public
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
        return LoginResponse{}, web.Unauthorized("identifiants invalides")
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

// 2. Contrôleur protégé — nécessite un JWT
type ProfileController struct {
    helix.Controller
    UserSvc *UserService `inject:"true"`
}

//helix:guard auth
func (c *ProfileController) Show(ctx web.Context) (UserProfile, error) {
    claims := ctx.Locals("claims").(map[string]any)
    return c.UserSvc.GetProfile(claims["sub"].(string))
}

// 3. Contrôleur admin — nécessite JWT + rôle admin
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

Configurez JWT via `config/application.yaml` :

```yaml
security:
  jwt:
    secret: "changez-moi-en-production-utilisez-une-longue-chaîne-aléatoire"
    expiry: "24h"
```

Le starter de sécurité lit ces valeurs et crée le `JWTService` automatiquement.

::: warning
Ne committez jamais de vrais secrets JWT dans le contrôle de version. Utilisez des variables d'environnement en production :
```bash
SECURITY_JWT_SECRET=votre-secret go run main.go
```
:::

## Hashage des mots de passe

Helix ne hache pas les mots de passe pour vous — c'est intentionnel. Utilisez `bcrypt` depuis la bibliothèque standard :

```go
import "golang.org/x/crypto/bcrypt"

// À la création d'un utilisateur :
hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
if err != nil {
    return nil, err
}
user.PasswordHash = string(hash)

// À l'authentification :
err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plainPassword))
if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
    return nil, web.Unauthorized("identifiants invalides")
}
```

Ne stockez jamais de mots de passe en clair. Ne les loguez jamais.

## Guards personnalisés

Implémentez `web.Guard` pour une logique d'autorisation custom :

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
    return web.Forbidden("IP non dans la liste d'autorisation")
}

web.RegisterGuard(server, "ip-allowlist", &IPAllowlistGuard{
    allowed: []string{"10.0.0.1", "10.0.0.2"},
})
```

```go
//helix:guard ip-allowlist
func (c *InternalController) Metrics(_ web.Context) (MetricsDump, error) { ... }
```

## Réponses d'erreur

| Scénario | Statut | Corps |
|----------|--------|-------|
| Token manquant | 401 | `{"error":{"type":"Unauthorized","message":"..."}}` |
| Token invalide/expiré | 401 | `{"error":{"type":"Unauthorized","message":"..."}}` |
| Rôle insuffisant | 403 | `{"error":{"type":"Forbidden","message":"..."}}` |
| Renouvellement (expiré) | 401 | `{"error":{"type":"Unauthorized","message":"token expired"}}` |
