# Helix

[![CI](https://github.com/enokdev/helix/actions/workflows/ci.yml/badge.svg)](https://github.com/enokdev/helix/actions/workflows/ci.yml)
[![Coverage](https://github.com/enokdev/helix/actions/workflows/coverage.yml/badge.svg)](https://github.com/enokdev/helix/actions/workflows/coverage.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/enokdev/helix)](https://goreportcard.com/report/github.com/enokdev/helix)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Helix est un framework backend Go inspire de Spring Boot, concu pour creer des APIs avec moins de boilerplate tout en restant idiomatique. Il combine injection de dependances, HTTP declaratif, repository generique, configuration centralisee, observabilite, securite, scheduling et CLI de developpement.

## Sommaire

- [Helix](#helix)
	- [Sommaire](#sommaire)
	- [Installation](#installation)
	- [Quick Start](#quick-start)
		- [Creer votre premiere application Helix](#creer-votre-premiere-application-helix)
		- [Explorer l'exemple complet](#explorer-lexemple-complet)
	- [Fonctionnalites](#fonctionnalites)
	- [Exemples](#exemples)
	- [Guides](#guides)
	- [Developpement du framework](#developpement-du-framework)
	- [Licence](#licence)

## Installation

Prerequis

- Go 1.21 ou plus recent
- Un module Go applicatif

```bash
go version
go mod init example.com/helix-users
go get github.com/enokdev/helix
```

Helix est publie comme module Go. `go get github.com/enokdev/helix` ajoute le framework a votre `go.mod` et telecharge les dependances necessaires.

## Quick Start

### Creer votre premiere application Helix

```bash
mkdir helix-users && cd helix-users
go mod init example.com/helix-users
go get github.com/enokdev/helix
```

Helix structure un service backend en trois types de composants :

```go
type UserRepository struct {
	helix.Repository
}

type UserService struct {
	helix.Service
	repo *UserRepository
}

type UserController struct {
	helix.Controller
	service *UserService
}

func (c *UserController) Index() []User {
	return c.service.List()
}

func main() {
	server := web.NewServer()
	ctrl := NewUserController(NewUserService(NewUserRepository()))
	if err := web.RegisterController(server, ctrl); err != nil {
		log.Fatal(err)
	}
	if err := helix.Run(helix.App{
		Components: []any{&appServer{server: server, addr: ":8080"}},
	}); err != nil {
		log.Fatal(err)
	}
}
```

`helix.Run` gere le cycle de vie complet : demarrage, ecoute de SIGTERM/SIGINT et arret propre.
Copiez la structure de `examples/crud-api` et adaptez les types a votre domaine.

### Explorer l'exemple complet

Une API CRUD `users` en memoire est disponible dans ce depot :

```bash
git clone https://github.com/enokdev/helix.git
cd helix
go run ./examples/crud-api
```

Dans un autre terminal :

```bash
curl http://localhost:8080/users
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}'
curl http://localhost:8080/users/1
curl -X PUT http://localhost:8080/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Byron","email":"ada.byron@example.com"}'
curl -X DELETE http://localhost:8080/users/1
```

## Fonctionnalites

- Injection de dependances via `helix.Service`, `helix.Controller`, `helix.Repository` et `helix.Component`.
- Bootstrap applicatif via `helix.Run(helix.App{...})` lorsque l'application assemble ses composants dans le conteneur.
- Resolution DI par reflection par defaut, avec mode wire pour le codegen compile-time.
- Routing HTTP par conventions `Index`, `Show`, `Create`, `Update`, `Delete` et directives `//helix:route`.
- Binding type des query params et body JSON, validation automatique et mapping retour vers status HTTP.
- Repository generique `data.Repository[T, ID, TX]` et adaptateur GORM via `data/gorm.NewRepository`.
- Tests d'application avec `helix.NewTestApp`, `helix.GetBean[T]` et `helix.MockBean[T]`.
- Configuration YAML, profils et variables d'environnement avec priorite `ENV > profile > application.yaml > default`.
- Endpoints d'observabilite `/actuator/health`, `/actuator/metrics` et `/actuator/info`.
- Securite JWT/RBAC, guards declaratifs, scheduling cron et CLI `helix`.

## Exemples

- [API CRUD users](examples/crud-api/main.go) : service, repository en memoire, controller declaratif et serveur HTTP Helix.

Commandes utiles :

```bash
go test ./examples/crud-api
go run ./examples/crud-api
```

## Guides

- [DI et configuration](docs/di-and-config.md)
- [Couche HTTP](docs/http-layer.md)
- [Data layer](docs/data-layer.md)
- [Securite, observabilite et scheduling](docs/security-observability-scheduling.md)

## Developpement du framework

Pour contribuer a Helix lui-meme :

```bash
git clone https://github.com/enokdev/helix.git
cd helix
go mod tidy
go test ./...
go build ./...
```

Avant d'ouvrir une pull request :

```bash
golangci-lint run
```

Si `golangci-lint` n'est pas installe localement, lancez au minimum :

```bash
go vet ./...
```

## Licence

MIT - voir [LICENSE](LICENSE).
