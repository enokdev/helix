# Configuration

Helix uses a Viper-backed configuration loader with a predictable priority chain and profile support.

## Priority Chain

```
ENV variables  >  application-{profile}.yaml  >  application.yaml  >  defaults
```

Higher sources always win. A `DATABASE_URL` environment variable overrides `database.url` in your YAML file.

## Creating a Loader

```go
import "github.com/enokdev/helix/config"

loader := config.NewLoader(
    config.WithConfigPaths("config", "."),      // search directories
    config.WithDefaults(map[string]any{
        "server.port": "8080",
    }),
    config.WithProfiles("dev"),                 // explicit profile activation
    config.WithEnvPrefix("APP"),                // APP_DATABASE_URL → database.url
    config.WithAllowMissingConfig(),            // don't error if YAML is absent
)
```

When using `helix.Run`, a loader is created automatically and shared with all starters.

## Config File

Place your config file at `config/application.yaml` (or pass custom paths via `WithConfigPaths`):

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
    secret: "change-me-in-production"
    expiry: "24h"

logging:
  level: info
  levels:
    "my-api/data": debug
```

## Loading into Structs

Use `mapstructure` tags to bind config into Go structs:

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

## Scalar Lookups

Retrieve individual values without a struct:

```go
port, ok := loader.Lookup("server.port")  // returns (any, bool)
if ok {
    fmt.Println(port) // "8080"
}

all := loader.AllSettings() // map[string]any deep copy
profiles := loader.ActiveProfiles() // []string
file := loader.ConfigFileUsed()     // path to loaded application.yaml
```

## Profiles

Profiles let you maintain environment-specific overrides without duplicating config.

### Activating profiles

**Via environment variable:**
```bash
HELIX_PROFILES_ACTIVE=prod go run main.go
```

**Via code:**
```go
loader := config.NewLoader(
    config.WithProfiles("dev", "local"),
)
```

**Multiple profiles** (comma-separated in env):
```bash
HELIX_PROFILES_ACTIVE=prod,feature-x go run main.go
```

### Profile files

Create `config/application-{profile}.yaml` — it is **merged** on top of `application.yaml`:

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

## Environment Variables

All config keys map to environment variables. Key transformations:

- `.` → `_`
- `-` → `_`
- Uppercase

Examples:
| Config key | Environment variable |
|-----------|---------------------|
| `server.port` | `SERVER_PORT` |
| `database.url` | `DATABASE_URL` |
| `security.jwt.secret` | `SECURITY_JWT_SECRET` |

With a prefix (`WithEnvPrefix("APP")`):
| Config key | Environment variable |
|-----------|---------------------|
| `server.port` | `APP_SERVER_PORT` |

## Dynamic Reload

Components can react to configuration changes at runtime.

### Enable reload

```yaml
# config/application.yaml
helix:
  config:
    reload-interval: 30s   # poll every 30 seconds
```

Send `SIGHUP` to trigger an immediate reload:

```bash
kill -HUP $(pgrep my-api)
```

### React to reloads

Implement `config.Reloadable` in any component:

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

## Injecting Config into Components

The config loader is available for field injection:

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

Or use `value:""` tags with `WithValueLookup`:

```go
type EmailService struct {
    helix.Service
    Host string `value:"email.smtp.host"`
    Port int    `value:"email.smtp.port"`
}
```
