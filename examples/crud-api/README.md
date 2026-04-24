# Helix CRUD API

Example complet d'API CRUD `users` avec les couches Helix principales:

- `UserRepository` garde les donnees en memoire et embed `helix.Repository`.
- `UserService` orchestre la logique applicative et embed `helix.Service`.
- `UserController` expose les routes HTTP conventionnelles et embed `helix.Controller`.
- `config/application.yaml` configure le port HTTP.

Les donnees sont volatiles: elles sont perdues a chaque redemarrage.

## Lancer

Depuis la racine du depot:

```bash
go run ./examples/crud-api
```

Le serveur ecoute sur `:8080`, valeur chargee depuis `examples/crud-api/config/application.yaml`.

## Tester

```bash
go test ./examples/crud-api
```

## Appeler l'API

Creer un utilisateur avec `POST /users`:

```bash
curl -i -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}'
```

Lister les utilisateurs avec `GET /users`:

```bash
curl -i http://localhost:8080/users
```

Lire un utilisateur avec `GET /users/:id`:

```bash
curl -i http://localhost:8080/users/1
```

Mettre a jour un utilisateur avec `PUT /users/:id`:

```bash
curl -i -X PUT http://localhost:8080/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Byron","email":"ada.byron@example.com"}'
```

Supprimer un utilisateur avec `DELETE /users/:id`:

```bash
curl -i -X DELETE http://localhost:8080/users/1
```
