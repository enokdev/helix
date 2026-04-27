# DI et configuration

Ce guide explique le conteneur d'injection de dependances Helix, les marqueurs de composants et le chargement de configuration. Il se concentre sur les APIs publiques actuellement disponibles et signale les limites utiles a connaitre.

## Sommaire

- [Modele mental](#modele-mental)
- [Marqueurs de composants](#marqueurs-de-composants)
- [Injection de dependances](#injection-de-dependances)
- [Scopes](#scopes)
- [Configuration](#configuration)
- [Profils](#profils)
- [Rechargement dynamique](#rechargement-dynamique)
- [Tests](#tests)
- [Erreurs frequentes](#erreurs-frequentes)

## Modele mental

Helix separe deux niveaux :

- Le package racine `helix` fournit les marqueurs applicatifs, `helix.Run` et `helix.App`.
- Le package `core` fournit le conteneur bas niveau : `core.Container`, `core.Resolver`, `core.ReflectResolver` et `core.WireResolver`.

`helix.Run(helix.App{Components: ...})` valide que les composants applicatifs portent un marqueur Helix. Le package `core`, lui, travaille au niveau plus bas : il enregistre surtout des pointeurs de structs et resout leurs dependances par type.

En mode reflection, les composants sont fournis comme valeurs deja creees. `helix.App.Scan` peut detecter des marqueurs dans des fichiers Go, mais il ne peut pas instancier seul les types trouves ; fournissez les valeurs dans `Components`.

```go
err := helix.Run(helix.App{
	Components: []any{
		&UserRepository{},
		&UserService{},
		&UserController{},
	},
})
```

## Marqueurs de composants

Les marqueurs de `helix` sont des embeds qui expriment l'intention d'un type :

- `helix.Service` : logique applicative et orchestration metier.
- `helix.Controller` : entree HTTP ou adaptateur de presentation.
- `helix.Repository` : acces aux donnees ou persistance.
- `helix.Component` : composant generique qui ne rentre pas dans les categories precedentes.

Exemple :

```go
type UserRepository struct {
	helix.Repository
}

type UserService struct {
	helix.Service
	Repository *UserRepository `inject:"true"`
}

type UserController struct {
	helix.Controller
	Service *UserService `inject:"true"`
}
```

Ces marqueurs sont verifies par `helix.Run`. Pour un usage direct de `core.NewContainer`, le conteneur exige surtout des pointeurs vers des structs enregistrables.

## Injection de dependances

Le resolver reflection injecte les champs exportes marques `inject:"true"`. Le type du champ determine la dependance a resoudre.

```go
container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))

_ = container.Register(&UserRepository{})
_ = container.Register(&UserService{})

var service *UserService
if err := container.Resolve(&service); err != nil {
	return err
}
```

Contraintes importantes :

- Le champ doit etre exporte. Un champ non exporte avec `inject:"true"` retourne `core.ErrUnresolvable`.
- Le champ doit etre assignable avec la valeur resolue.
- Une dependance absente retourne `core.ErrNotFound`.
- Une cible de resolution invalide ou une ambiguite retourne `core.ErrUnresolvable`.
- Un cycle de dependances retourne `CyclicDepError`.

### Resolution par interface

Un champ peut demander une interface. Le resolver accepte ce cas si exactement un composant enregistre est assignable a cette interface.

```go
type Mailer interface {
	Send(to string, body string) error
}

type SMTPMailer struct {
	helix.Component
}

func (m *SMTPMailer) Send(to string, body string) error {
	return nil
}

type SignupService struct {
	helix.Service
	Mailer Mailer `inject:"true"`
}
```

Si deux implementations de `Mailer` sont enregistrees, Helix refuse de choisir arbitrairement et retourne `core.ErrUnresolvable`.

### Graphe de dependances

Chaque `core.Resolver` expose `Graph()`. En mode reflection, le graphe est enrichi quand les champs `inject:"true"` sont resolus.

```go
graph := containerGraph.Graph()
_ = graph.Nodes
_ = graph.Edges
```

Dans un guide ou un outil, utilisez ce graphe comme aide au diagnostic, pas comme contrat metier.

### Injection de valeurs

Le tag `value:"key"` injecte une valeur scalaire depuis la configuration.

En **mode zero-config** (`helix.Run()` sans argument), la config auto-chargee est automatiquement branchee au container — les champs `value:"key"` sont resolus sans aucun code supplementaire.

En **mode explicite**, fournissez `core.WithValueLookup(loader.Lookup)` au container :

```go
type HTTPConfig struct {
	helix.Component
	Port int `value:"server.port"`
}

loader := config.NewLoader(
	config.WithAllowMissingConfig(),
	config.WithDefaults(map[string]any{"server.port": 8080}),
)
if err := loader.Load(new(struct{})); err != nil {
	return err
}

container := core.NewContainer(
	core.WithResolver(core.NewReflectResolver()),
	core.WithValueLookup(loader.Lookup),
)
```

Le resolver convertit les valeurs scalaires courantes vers le type du champ quand c'est possible. Gardez les exemples simples : `string`, `bool`, entiers et flottants.

## Scopes

Helix supporte deux scopes :

- `core.ScopeSingleton` : comportement par defaut. Chaque `Resolve` retourne la meme instance.
- `core.ScopePrototype` : chaque `Resolve` cree une nouvelle instance zero-value, puis reinjecte ses champs `inject:"true"` et `value:"..."`.

Utilisez `core.NewComponentRegistration` pour obtenir les defaults surs.

```go
registration := core.NewComponentRegistration(&UserService{})
registration.Scope = core.ScopeSingleton
```

Pour un prototype :

```go
registration := core.ComponentRegistration{
	Component: &RequestState{},
	Scope:     core.ScopePrototype,
}
```

Point subtil : avec `core.ScopePrototype`, les champs non injectes du composant source ne sont pas copies. Le resolver alloue une nouvelle instance zero-value a chaque resolution.

### Lazy

`Lazy` n'est pas un scope. C'est un flag de `core.ComponentRegistration` qui retarde la materialisation d'un singleton jusqu'au premier `Resolve`.

```go
registration := core.ComponentRegistration{
	Component: &ExpensiveService{},
	Scope:     core.ScopeSingleton,
	Lazy:      true,
}
```

Les composants lazy sont ignores par `Container.Start()` pour le lifecycle. `helix.Run` rejette aussi la combinaison `core.ScopePrototype` + `Lazy`, car un prototype lazy n'a pas de sens stable dans le cycle de vie actuel.

## Configuration

Le loader de configuration vit dans le package `config`. Il charge `application.yaml`, merge les profils, applique les defaults et expose `Lookup` pour l'injection `value:"key"`.

La priorite effective est :

```text
ENV > profil YAML > application.yaml > DEFAULT
```

Exemple de struct :

```go
type AppConfig struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
}
```

`mapstructure:"key"` est le tag a utiliser pour lier les champs Go aux cles YAML.

Exemple `config/application.yaml` :

```yaml
server:
  port: 8080
app:
  name: helix-api
```

Chargement :

```go
loader := config.NewLoader(
	config.WithConfigPaths("config"),
	config.WithDefaults(map[string]any{
		"server.port": 7070,
		"app.name":    "default-name",
	}),
)

var cfg AppConfig
if err := loader.Load(&cfg); err != nil {
	return err
}

port, ok := loader.Lookup("server.port")
_ = port
_ = ok
```

Options utiles :

- `config.WithConfigPaths("config")` : dossiers cherches pour `application.yaml`.
- `config.WithAllowMissingConfig()` : autorise un chargement depuis defaults, profils et ENV sans fichier base.
- `config.WithDefaults(map[string]any{...})` : valeurs DEFAULT.
- `config.WithProfiles("dev")` : profils explicites.
- `config.WithEnvPrefix("HELIX")` : prefixe les variables d'environnement.

Autres methodes :

- `Loader.Lookup(key)` : valeur resolue apres `Load`.
- `Loader.AllSettings()` : copie defensive de toutes les valeurs.
- `Loader.ActiveProfiles()` : profils actifs.
- `Loader.ConfigFileUsed()` : chemin du `application.yaml` charge.

### Variables d'environnement

Helix configure Viper avec `AutomaticEnv` et un replacer qui transforme `.` et `-` en `_`.

Exemples :

- `server.port` -> `SERVER_PORT`
- `helix.config.reload-interval` -> `HELIX_CONFIG_RELOAD_INTERVAL`
- avec `config.WithEnvPrefix("HELIX")`, `server.port` -> `HELIX_SERVER_PORT`

## Profils

Un profil charge un fichier `application-<profile>.yaml` par-dessus `application.yaml`.

Exemple `config/application-dev.yaml` :

```yaml
server:
  port: 8081
app:
  name: helix-api-dev
```

Activation explicite :

```go
loader := config.NewLoader(
	config.WithConfigPaths("config"),
	config.WithProfiles("dev"),
)
```

Activation par environnement :

```bash
HELIX_PROFILES_ACTIVE=dev go run ./cmd/app
```

Plusieurs profils sont separes par des virgules. Les profils manquants sont ignores, ce qui permet de garder des profils locaux optionnels.

## Rechargement dynamique

`Loader.Load` ne demarre aucun reload automatique. Le reload est opt-in avec `config.NewReloader`.

```go
type CacheSettings struct {
	TTL string `mapstructure:"ttl"`
}

type CacheService struct {
	helix.Service
}

func (s *CacheService) OnConfigReload() {
	// Recalculer les caches ou relire une valeur deja chargee.
}

reloader, err := config.NewReloader(
	loader,
	&cacheSettings,
	config.WithReloadables(cacheService),
	config.WithReloadInterval(30*time.Second),
)
if err != nil {
	return err
}
```

`config.NewReloader` expose :

- `Reload()` : recharge une fois sous verrou.
- `Start(ctx)` : ecoute les signaux et le polling configure.
- `RLock()` / `RUnlock()` : verrou de lecture a tenir pendant la lecture de la struct partagee.
- `config.WithReloadables(...)` : appelle `OnConfigReload` apres un reload reussi.
- `config.WithReloadInterval(...)` : intervalle de polling explicite.
- `config.WithReloadLogger(...)` : logger pour les erreurs en arriere-plan.

Sur Unix, Helix ecoute `SIGHUP` via `reload_signal_unix.go`. Sur les plateformes non Unix, la source de signal peut etre vide ; utilisez `config.WithReloadInterval` pour un comportement portable.

Si un reload echoue, les tests couvrent la conservation de la configuration precedente dans les cas d'erreur de lecture ou de decode. Ne supposez pas de mutation partielle.

Lecture sans data race :

```go
reloader.RLock()
currentPort := cfg.Server.Port
reloader.RUnlock()
_ = currentPort
```

Le type racine expose aussi `helix.ConfigReloadable`, alias de `config.Reloadable`, pour les composants applicatifs qui veulent signaler `OnConfigReload`.

## Tests

Pour tester un composant avec DI et configuration, utilisez les helpers racine :

```go
app := helix.NewTestApp(t,
	helix.TestConfigDefaults(map[string]any{
		"server.port": 5050,
	}),
	helix.TestComponents(
		&UserRepository{},
		&UserService{},
	),
)

service := helix.GetBean[*UserService](app)
_ = service
```

Helpers utiles :

- `helix.TestComponents(...)` : enregistre des composants.
- `helix.TestConfigDefaults(...)` : fournit des valeurs par defaut.
- `helix.TestConfigPaths(...)` : pointe vers des fichiers de config de test.
- `helix.TestContainerOptions(...)` : ajoute des options `core`.
- `helix.MockBean[T](impl)` : remplace une dependance assignable a `T`.

## Erreurs frequentes

- Oublier le pointeur dans `Register` : `Register(UserService{})` echoue, utilisez `Register(&UserService{})`.
- Mettre `inject:"true"` sur un champ non exporte : le resolver ne peut pas l'assigner.
- Enregistrer deux implementations pour la meme interface et demander cette interface : resolution ambigue, `core.ErrUnresolvable`.
- Attendre que `helix.App.Scan` instancie les types : le scan detecte, mais les valeurs doivent etre fournies en mode reflection.
- Utiliser `value:"key"` sans `core.WithValueLookup(loader.Lookup)` en mode explicite : la valeur ne peut pas etre trouvee. En mode zero-config (`helix.Run()`), ce branchage est automatique.
- Confondre `Lazy` et `core.ScopePrototype` : `Lazy` retarde un singleton, `core.ScopePrototype` cree une nouvelle instance a chaque resolution.
- Lire une struct rechargee sans `RLock/RUnlock` pendant que `config.NewReloader` tourne en arriere-plan.
