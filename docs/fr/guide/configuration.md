# Configuration

Helix utilise un loader de configuration basé sur Viper, avec une chaîne de priorité prévisible et le support des profils.

## Chaîne de priorité

```
Variables ENV  >  application-{profil}.yaml  >  application.yaml  >  valeurs par défaut
```

Les sources de priorité supérieure l'emportent toujours. Une variable d'environnement `DATABASE_URL` écrase `database.url` dans votre fichier YAML.

## Créer un Loader

```go
import "github.com/enokdev/helix/config"

loader := config.NewLoader(
    config.WithConfigPaths("config", "."),      // répertoires de recherche
    config.WithDefaults(map[string]any{
        "server.port": "8080",
    }),
    config.WithProfiles("dev"),                 // activation explicite de profil
    config.WithEnvPrefix("APP"),                // APP_DATABASE_URL → database.url
    config.WithAllowMissingConfig(),            // pas d'erreur si le YAML est absent
)
```

Avec `helix.Run`, un loader est créé automatiquement et partagé avec tous les starters.

## Fichier de config

Placez votre fichier de config dans `config/application.yaml` (ou passez des chemins personnalisés via `WithConfigPaths`) :

```yaml
# config/application.yaml
server:
  port: 8080

app:
  name: my-api
  version: "1.0.0"

database:
  url: "my-api.db"
  pool:
    max-open: 25
    max-idle: 5

security:
  jwt:
    secret: "changez-en-production"
    expiry: "24h"

logging:
  level: info
  levels:
    "my-api/data": debug
```

## Chargement dans des structs

Utilisez les tags `mapstructure` pour lier la config dans des structs Go :

```go
type AppConfig struct {
    Server struct {
        Port string `mapstructure:"port"`
    } `mapstructure:"server"`
    App struct {
        Name    string `mapstructure:"name"`
        Version string `mapstructure:"version"`
    } `mapstructure:"app"`
}

var cfg AppConfig
if err := loader.Load(&cfg); err != nil {
    log.Fatal(err)
}

fmt.Println(cfg.Server.Port) // "8080"
```

## Lookups scalaires

Récupérez des valeurs individuelles sans struct :

```go
port, ok := loader.Lookup("server.port")  // retourne (any, bool)
if ok {
    fmt.Println(port) // "8080"
}

all := loader.AllSettings() // copie profonde map[string]any
profiles := loader.ActiveProfiles() // []string
file := loader.ConfigFileUsed()     // chemin vers application.yaml chargé
```

## Profils

Les profils permettent de maintenir des surcharges spécifiques à l'environnement sans dupliquer la config.

### Activer les profils

**Via variable d'environnement :**
```bash
HELIX_PROFILES_ACTIVE=prod go run main.go
```

**Via le code :**
```go
loader := config.NewLoader(
    config.WithProfiles("dev", "local"),
)
```

**Profils multiples** (séparés par des virgules dans l'env) :
```bash
HELIX_PROFILES_ACTIVE=prod,feature-x go run main.go
```

### Fichiers de profil

Créez `config/application-{profil}.yaml` — il est **fusionné** par-dessus `application.yaml` :

```yaml
# config/application-prod.yaml
server:
  port: 443

database:
  url: "postgres://..."
  pool:
    max-open: 100

logging:
  level: warn
```

```yaml
# config/application-dev.yaml
logging:
  level: debug

database:
  url: "dev.db"
```

## Variables d'environnement

Toutes les clés de config se mappent à des variables d'environnement. Transformations des clés :

- `.` → `_`
- `-` → `_`
- Majuscules

Exemples :
| Clé de config | Variable d'environnement |
|--------------|--------------------------|
| `server.port` | `SERVER_PORT` |
| `database.url` | `DATABASE_URL` |
| `security.jwt.secret` | `SECURITY_JWT_SECRET` |

Avec un préfixe (`WithEnvPrefix("APP")`) :
| Clé de config | Variable d'environnement |
|--------------|--------------------------|
| `server.port` | `APP_SERVER_PORT` |

## Rechargement dynamique

Les composants peuvent réagir aux changements de configuration à l'exécution.

### Activer le rechargement

```yaml
# config/application.yaml
helix:
  config:
    reload-interval: 30s   # sondage toutes les 30 secondes
```

Envoyez `SIGHUP` pour déclencher un rechargement immédiat :

```bash
kill -HUP $(pgrep my-api)
```

### Réagir aux rechargements

Implémentez `config.Reloadable` dans n'importe quel composant :

```go
type FeatureFlags struct {
    helix.Component
    mu      sync.RWMutex
    enabled bool
    loader  config.Loader
}

func (f *FeatureFlags) OnConfigReload() {
    f.mu.Lock()
    defer f.mu.Unlock()
    val, _ := f.loader.Lookup("features.new-checkout")
    f.enabled, _ = val.(bool)
}

func (f *FeatureFlags) NewCheckoutEnabled() bool {
    f.mu.RLock()
    defer f.mu.RUnlock()
    return f.enabled
}
```

## Fichiers TOML et JSON

Helix supporte les formats YAML, TOML et JSON. Le format est détecté depuis l'extension :

```toml
# config/application.toml
[server]
port = "8080"

[app]
name = "my-api"
version = "1.0.0"

[database]
url = "app.db"

[database.pool]
max-open = 25
max-idle = 5

[security.jwt]
secret = "changez-moi"
expiry = "24h"

[logging]
level = "info"
```

```json
// config/application.json
{
  "server": { "port": "8080" },
  "app": { "name": "my-api", "version": "1.0.0" },
  "database": { "url": "app.db", "pool": { "max-open": 25, "max-idle": 5 } },
  "security": { "jwt": { "secret": "changez-moi", "expiry": "24h" } },
  "logging": { "level": "info" }
}
```

## Config imbriquée complexe

Liez une config profondément imbriquée dans des structs Go :

```go
type AppConfig struct {
    Server struct {
        Port         string        `mapstructure:"port"`
        ReadTimeout  time.Duration `mapstructure:"read-timeout"`
        WriteTimeout time.Duration `mapstructure:"write-timeout"`
    } `mapstructure:"server"`

    Database struct {
        URL  string `mapstructure:"url"`
        Pool struct {
            MaxOpen     int           `mapstructure:"max-open"`
            MaxIdle     int           `mapstructure:"max-idle"`
            MaxLifetime time.Duration `mapstructure:"max-lifetime"`
        } `mapstructure:"pool"`
    } `mapstructure:"database"`

    Email struct {
        SMTP struct {
            Host     string `mapstructure:"host"`
            Port     int    `mapstructure:"port"`
            Username string `mapstructure:"username"`
        } `mapstructure:"smtp"`
        From string `mapstructure:"from"`
    } `mapstructure:"email"`
}

var cfg AppConfig
if err := loader.Load(&cfg); err != nil {
    log.Fatal(err)
}
```

```yaml
server:
  port: "8080"
  read-timeout: 30s
  write-timeout: 30s

email:
  from: "noreply@example.com"
  smtp:
    host: "smtp.example.com"
    port: 587
    username: "noreply@example.com"
```

## Validation de config au démarrage

Validez les valeurs de config requises avant le démarrage de l'application :

```go
type ConfigValidator struct {
    helix.Component
    Loader config.Loader `inject:"true"`
}

func (v *ConfigValidator) OnStart() error {
    required := []string{
        "server.port",
        "database.url",
        "security.jwt.secret",
    }

    for _, key := range required {
        val, ok := v.Loader.Lookup(key)
        if !ok || val == "" {
            return fmt.Errorf("la clé de config requise %q est manquante ou vide", key)
        }
    }

    // Valider la robustesse du secret JWT :
    secret, _ := v.Loader.Lookup("security.jwt.secret")
    if len(fmt.Sprint(secret)) < 32 {
        return fmt.Errorf("security.jwt.secret doit contenir au moins 32 caractères")
    }

    return nil
}
```

Enregistrez-le comme premier composant — `OnStart` s'exécute dans l'ordre des dépendances, donc la validation se fait avant le démarrage des autres composants :

```go
helix.Run(helix.App{
    Components: []any{
        &ConfigValidator{},
        &UserRepository{},
        // ...
    },
})
```

## Masquage des secrets dans les logs

Les valeurs de config ne doivent jamais apparaître dans les logs. Utilisez une fonction de masquage pour déboguer la config :

```go
func safeDump(loader config.Loader) map[string]any {
    all := loader.AllSettings()
    mask(all, "secret", "password", "key", "token")
    return all
}

func mask(m map[string]any, keys ...string) {
    for k, v := range m {
        lower := strings.ToLower(k)
        for _, sensitive := range keys {
            if strings.Contains(lower, sensitive) {
                m[k] = "***"
                break
            }
        }
        if nested, ok := v.(map[string]any); ok {
            mask(nested, keys...)
        }
    }
}

// Au démarrage :
slog.Debug("config chargée", "settings", safeDump(loader))
```

## Injection de config dans les composants

Le loader de config est disponible pour l'injection de champ :

```go
type EmailService struct {
    helix.Service
    Config config.Loader `inject:"true"`
}

func (s *EmailService) SMTPHost() string {
    v, _ := s.Config.Lookup("email.smtp.host")
    host, _ := v.(string)
    return host
}
```

Ou utilisez les tags `value:""` avec `WithValueLookup` :

```go
type EmailService struct {
    helix.Service
    Host string `value:"email.smtp.host"`
    Port int    `value:"email.smtp.port"`
}
```
