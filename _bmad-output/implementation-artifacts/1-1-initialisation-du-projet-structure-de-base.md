# Story 1.1: Initialisation du Projet & Structure de Base

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

En tant que **contributeur du framework**,
Je veux initialiser le module Go Helix avec la structure de packages et l'outillage CI,
Afin d'avoir une base solide sur laquelle construire le framework.

## Acceptance Criteria

1. **Given** un répertoire vide
   **When** `go mod init github.com/enokdev/helix` est exécuté et les fichiers initiaux sont créés
   **Then** le module compile avec `go build ./...` sans erreur

2. **And** `golangci-lint run` passe sans erreur

3. **And** `go test ./...` s'exécute (0 tests, 0 erreurs)

4. **And** GitHub Actions CI tourne sur push et PR (lint + test + build)

5. **And** la structure de dossiers suivante existe :
   `core/`, `config/`, `web/`, `data/`, `testutil/`, `starter/`, `observability/`, `security/`, `scheduler/`, `cli/`, `examples/`

## Tasks / Subtasks

- [x] Tâche 1 : Initialiser le module Go (AC: #1)
  - [x] Exécuter `go mod init github.com/enokdev/helix`
  - [x] Vérifier `go 1.21` dans `go.mod`
  - [x] Créer `go.sum` (vide à ce stade)

- [x] Tâche 2 : Créer la structure de packages (AC: #5)
  - [x] Créer `core/` avec fichier placeholder `doc.go` (package core)
  - [x] Créer `config/` avec fichier placeholder `doc.go` (package config)
  - [x] Créer `web/` avec fichier placeholder `doc.go` (package web)
  - [x] Créer `web/internal/` avec `doc.go` (package internal)
  - [x] Créer `data/` avec fichier placeholder `doc.go` (package data)
  - [x] Créer `data/gorm/` avec `doc.go` (package gorm)
  - [x] Créer `testutil/` avec fichier placeholder `doc.go` (package testutil)
  - [x] Créer `starter/` avec `doc.go` (package starter)
  - [x] Créer `observability/` avec `doc.go` (package observability)
  - [x] Créer `security/` avec `doc.go` (package security)
  - [x] Créer `scheduler/` avec `doc.go` (package scheduler)
  - [x] Créer `cli/` avec `doc.go` (package cli)
  - [x] Créer `examples/crud-api/` avec `main.go` vide
  - [x] Créer `helix.go` à la racine (package helix — marqueurs vides)

- [x] Tâche 3 : Configurer l'outillage de qualité (AC: #2, #3)
  - [x] Créer `.golangci.yml` avec les linters requis (vet, staticcheck, errcheck, gofumpt)
  - [x] Vérifier que `go build ./...` passe
  - [x] Vérifier que `go test ./...` s'exécute sans erreur

- [x] Tâche 4 : Créer le pipeline CI GitHub Actions (AC: #4)
  - [x] Créer `.github/workflows/ci.yml` (lint + test + build sur push et PR)
  - [x] Créer `.github/workflows/release.yml` (goreleaser sur tag `v*`)

- [x] Tâche 5 : Ajouter les fichiers racine du projet
  - [x] Créer `README.md` (titre + badges CI + instructions démarrage rapide)
  - [x] Créer `CONTRIBUTING.md`
  - [x] Créer `LICENSE` (MIT recommandé)
  - [x] Créer `.gitignore` (binaires Go, `.env`, `_bmad-output/`)

## Dev Notes

### Contexte Architectural

Cette story est le **Sprint 0** — fondation sur laquelle tout le framework repose. Aucun code fonctionnel n'est attendu : uniquement la structure, le module et le CI.

- **Module :** `github.com/enokdev/helix` (ou `github.com/{org}/helix` — utiliser `enokdev` par défaut)
- **Go minimum :** `go 1.21` — obligatoire pour `slog` (stdlib) et generics stables
- **Aucune dépendance externe** dans `go.mod` à ce stade — les dépendances (Fiber, GORM, Viper…) seront ajoutées dans les stories suivantes

### Structure de Packages à Créer

```
helix/
├── go.mod                          # module github.com/enokdev/helix, go 1.21
├── go.sum
├── helix.go                        # package helix — types marqueurs vides (Story 1.7)
├── README.md
├── CONTRIBUTING.md
├── LICENSE
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── .golangci.yml
├── core/
│   └── doc.go                      # "Package core provides the DI container..."
├── config/
│   └── doc.go
├── web/
│   ├── doc.go
│   └── internal/
│       └── doc.go
├── data/
│   ├── doc.go
│   └── gorm/
│       └── doc.go
├── testutil/
│   └── doc.go
├── starter/
│   └── doc.go
├── observability/
│   └── doc.go
├── security/
│   └── doc.go
├── scheduler/
│   └── doc.go
├── cli/
│   └── doc.go
└── examples/
    └── crud-api/
        └── main.go                 # package main vide
```

### Règles Critiques à Respecter

**Naming des packages :**
- Toujours minuscules, singulier, sans underscore ni tiret : `core`, `config`, `web`, `data`, `testutil` ✅
- INTERDIT : `Core`, `configs`, `web_layer` ❌

**`helix.go` à la racine (package `helix`) :**
- Ce fichier déclare les types marqueurs qui seront utilisés dans les stories suivantes
- À ce stade : contenu minimal, juste `package helix` avec commentaire de package
- Ne PAS implémenter `helix.Run()` ici — c'est la Story 1.7

**`doc.go` dans chaque package :**
```go
// Package core provides the dependency injection container and lifecycle management
// for the Helix framework.
package core
```

**Règle d'import stricte (à respecter dès le début) :**
- `core/` : ZÉRO import d'autres packages Helix
- `config/` : ZÉRO import d'autres packages Helix
- `web/internal/` : seul endroit autorisé à importer `gofiber/fiber` (pas encore ajouté à ce stade)

### Configuration `.golangci.yml`

```yaml
run:
  go: "1.21"
  timeout: 5m

linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - gofumpt
    - revive
    - unused

linters-settings:
  gofumpt:
    extra-rules: true
```

### Pipeline CI `.github/workflows/ci.yml`

```yaml
name: CI
on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  lint-test-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.21"
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
      - name: Test
        run: go test ./...
      - name: Build
        run: go build ./...
```

### Project Structure Notes

- Cette story ne crée QUE la structure et l'outillage — aucune logique métier
- La story 1.2 créera les interfaces publiques dans `core/`
- La story 1.7 créera `helix.Run()` et les marqueurs de composants dans `helix.go`
- Les dépendances Go (`fiber`, `gorm`, `viper`) ne sont PAS ajoutées dans cette story

### Alignement avec Architecture

- Structure conforme à [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Arborescence Complète du Projet"]
- Frontières d'import documentées dans [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Frontières Architecturales"]
- Outillage CI conforme aux décisions de [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Fondations du Projet & Outillage"]

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` — Story 1.1 Acceptance Criteria]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Fondations du Projet & Outillage"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Arborescence Complète du Projet"]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section "Patterns d'Implémentation & Règles de Cohérence" — Naming Patterns]

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.6 (claude-sonnet-4.6)

### Debug Log References

_Aucun blocage rencontré._

### Completion Notes List

- Toutes les tâches et sous-tâches sont complètes.
- `go build ./...` passe sans erreur (confirmé localement).
- `go test ./...` s'exécute avec 0 tests, 0 erreurs (comportement attendu — aucune logique métier à ce stade).
- `.golangci.yml` configuré avec `govet`, `staticcheck`, `errcheck`, `gofumpt`, `revive`, `unused` — sera validé par CI.
- La structure de packages respecte les règles de nommage idiomatic Go (minuscules, singulier, sans tiret/underscore).
- `helix.go` contient uniquement la déclaration du package — `helix.Run()` et les marqueurs seront ajoutés en Story 1.7.
- `.gitignore` exclut `_bmad-output/` pour ne pas versionner les artefacts de planification.

### File List

- `go.mod`
- `go.sum`
- `helix.go`
- `.golangci.yml`
- `.gitignore`
- `README.md`
- `CONTRIBUTING.md`
- `LICENSE`
- `core/doc.go`
- `config/doc.go`
- `web/doc.go`
- `web/internal/doc.go`
- `data/doc.go`
- `data/gorm/doc.go`
- `testutil/doc.go`
- `starter/doc.go`
- `observability/doc.go`
- `security/doc.go`
- `scheduler/doc.go`
- `cli/doc.go`
- `examples/crud-api/main.go`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (statut mis à jour)
- `_bmad-output/implementation-artifacts/1-1-initialisation-du-projet-structure-de-base.md` (story file)

### Change Log

- 2026-04-14 : Initialisation du module Go (`github.com/enokdev/helix`, go 1.21), création de toute la structure de packages avec `doc.go` placeholders, configuration `.golangci.yml`, pipeline CI GitHub Actions, fichiers racine (README, CONTRIBUTING, LICENSE, .gitignore).
