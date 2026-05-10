# Tests

Helix inclut des utilitaires de test de première classe qui démarrent un vrai conteneur DI — pas de mocks-de-mocks, pas de boilerplate de câblage.

## TestApp

`helix.NewTestApp` crée un conteneur entièrement câblé pour les tests :

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
        t.Fatal("ID non-zéro attendu")
    }
}
```

La test app est nettoyée automatiquement quand le test se termine.

## Options

### `helix.TestComponents`

Enregistrer des composants pré-instanciés :

```go
helix.TestComponents(
    &UserRepository{},
    &UserService{},
    &EmailService{},
)
```

### `helix.TestConfigPaths`

Charger la config depuis des répertoires personnalisés :

```go
helix.TestConfigPaths("testdata/config", "../../config")
```

### `helix.TestConfigDefaults`

Fournir des valeurs de config en code sans fichier :

```go
helix.TestConfigDefaults(map[string]any{
    "server.port":          "8181",
    "database.url":         ":memory:",
    "security.jwt.secret":  "test-secret",
})
```

### `helix.MockBean`

Remplacer un composant par un double de test :

```go
type MockEmailService struct{}

func (m *MockEmailService) Send(to, subject, body string) error {
    return nil // no-op dans les tests
}

app := helix.NewTestApp(t,
    helix.TestComponents(&UserService{}),
    helix.MockBean[*EmailService](&MockEmailService{}),
)
```

Le mock satisfait le même type que le composant réel et est injecté partout où `*EmailService` est demandé.

## Obtenir des composants

```go
svc  := helix.GetBean[*UserService](app)
repo := helix.GetBean[*UserRepository](app)
```

`GetBean` panique si le composant n'est pas trouvé — approprié pour les tests où un câblage manquant est toujours un bug.

## Tests de contrôleurs HTTP

Testez les contrôleurs de bout en bout sans démarrer un vrai serveur :

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

    // Requête HTTP en-process
    req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Bob","email":"bob@example.com"}`))
    req.Header.Set("Content-Type", "application/json")

    resp, err := server.ServeHTTP(req)
    if err != nil {
        t.Fatal(err)
    }

    if resp.StatusCode != 201 {
        t.Fatalf("201 attendu, obtenu %d", resp.StatusCode)
    }
}
```

`server.ServeHTTP` exécute la chaîne complète de handlers (binding, guards, interceptors) sans listener réseau.

## Tests HTTP avec authentification JWT

Testez les endpoints protégés en générant un token dans le test :

```go
func TestProfile_Authentifié(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(
            &UserRepository{},
            &UserService{},
            &ProfileController{},
        ),
        helix.TestConfigDefaults(map[string]any{
            "security.jwt.secret": "clé-secrète-test",
            "security.jwt.expiry": "1h",
        }),
    )

    // Générer un token de test :
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

    resp, _ := server.ServeHTTP(req)
    if resp.StatusCode != 200 {
        t.Fatalf("200 attendu, obtenu %d", resp.StatusCode)
    }
}

func TestProfile_NonAuthentifié(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.TestComponents(&ProfileController{}),
        helix.TestConfigDefaults(map[string]any{
            "security.jwt.secret": "clé-secrète-test",
        }),
    )

    server := helix.GetBean[web.HTTPServer](app)
    req := httptest.NewRequest("GET", "/profile", nil)
    // Pas d'en-tête Authorization

    resp, _ := server.ServeHTTP(req)
    if resp.StatusCode != 401 {
        t.Fatalf("401 attendu, obtenu %d", resp.StatusCode)
    }
}
```

## Tester le cycle de vie

```go
func TestDatabaseConnection_Lifecycle(t *testing.T) {
    db, _ := datagorm.OpenSQLite(":memory:")
    conn := &DatabaseConnection{db: db.DB()}

    // Tester OnStart
    if err := conn.OnStart(); err != nil {
        t.Fatalf("OnStart a échoué : %v", err)
    }

    // Tester OnStop
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := conn.OnStop(ctx); err != nil {
        t.Fatalf("OnStop a échoué : %v", err)
    }
}
```

## Tests parallèles

`helix.NewTestApp` est sûr à appeler depuis des tests parallèles — chacun crée son propre conteneur :

```go
func TestUserService(t *testing.T) {
    cases := []struct {
        name  string
        email string
        valid bool
    }{
        {"valide", "alice@example.com", true},
        {"email invalide", "pas-un-email", false},
        {"email vide", "", false},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()  // chaque sous-test crée sa propre app

            app := helix.NewTestApp(t,
                helix.TestComponents(&UserService{}),
                helix.TestConfigDefaults(map[string]any{
                    "database.url": ":memory:",
                }),
            )

            svc := helix.GetBean[*UserService](app)
            _, err := svc.Create(tc.email)

            if tc.valid && err != nil {
                t.Fatalf("aucune erreur attendue, obtenu %v", err)
            }
            if !tc.valid && err == nil {
                t.Fatal("erreur attendue pour une entrée invalide")
            }
        })
    }
}
```

## Données et fichiers de config de test

Placez la config de test dans un répertoire `testdata/` :

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
    secret: "secret-test-uniquement"
    expiry: "1h"
```

## Tests d'intégration

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

    // Créer
    user, err := svc.Create("Alice", "alice@example.com")
    require.NoError(t, err)
    require.NotZero(t, user.ID)

    // Lire
    found, err := svc.Get(user.ID)
    require.NoError(t, err)
    require.Equal(t, "Alice", found.Name)

    // Supprimer
    require.NoError(t, svc.Delete(user.ID))
    _, err = svc.Get(user.ID)
    require.ErrorIs(t, err, data.ErrRecordNotFound)
}
```

## Conseils

- Utilisez `TestConfigDefaults` plutôt que des fichiers pour les cas simples — plus rapide et plus portable
- Utilisez `MockBean` pour les dépendances externes lentes (email, SMS, S3) mais préférez les vraies implémentations pour les repositories
- `server.ServeHTTP` couvre toute la pile middleware — préférez-le à l'appel direct des méthodes de service pour les tests de contrôleur
- SQLite en mémoire (`:memory:`) vous donne une base de données fraîche par test sans overhead de nettoyage
- `t.Parallel()` est sûr — chaque `NewTestApp` crée un conteneur isolé
- Générez des tokens JWT dans les tests en utilisant le vrai `JWTService` — cela teste le flux auth de bout en bout
