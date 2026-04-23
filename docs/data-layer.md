# Data layer

Ce guide detaille le pattern Repository et l'acces aux donnees dans Helix.

## Objectif

Comprendre comment persister des entites avec `data.Repository[T, ID, TX]`, l'adaptateur GORM et les transactions declaratives.

## Sections prevues

- Interface `data.Repository[T, ID, TX]`
- Adaptateur `data/gorm.NewRepository`
- Filtres et pagination
- Transactions via contexte
- Directive `//helix:transactional`

> **Note CGO** : l'adaptateur SQLite (`github.com/mattn/go-sqlite3`) requiert CGO.
> Compilez avec `CGO_ENABLED=1` et un compilateur C disponible dans votre PATH.

## References rapides

- Package `data`
- Package `data/gorm`
