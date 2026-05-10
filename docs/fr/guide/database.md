# Base de données & Repository

Helix fournit un pattern repository générique avec un adaptateur GORM. Il maintient votre code de domaine découplé de l'ORM tout en vous donnant un accès complet à GORM quand c'est nécessaire.

## L'interface Repository

```go
// data/repository.go
type Repository[T any, ID any, TX any] interface {
    FindAll(ctx context.Context) ([]T, error)
    FindByID(ctx context.Context, id ID) (*T, error)
    FindWhere(ctx context.Context, filter Filter) ([]T, error)
    Save(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id ID) error
    Paginate(ctx context.Context, page, size int) (Page[T], error)
    WithTransaction(tx Transaction[TX]) Repository[T, ID, TX]
}
```

## Adaptateur GORM

### Définir votre modèle

```go
type Product struct {
    ID          uint    `gorm:"primaryKey"`
    Name        string  `gorm:"not null"`
    Price       float64
    CategoryID  uint
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Créer un repository

```go
import datagorm "github.com/enokdev/helix/data/gorm"

repo := datagorm.NewRepository[Product, uint](db)
```

C'est tout — vous avez maintenant `FindAll`, `FindByID`, `FindWhere`, `Save`, `Delete`, `Paginate`.

### Repository personnalisé

Intégrez le repository GORM dans votre repository de domaine pour l'étendre :

```go
type ProductRepository struct {
    helix.Repository
    *datagorm.Repository[Product, uint]
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
    return &ProductRepository{
        Repository: datagorm.NewRepository[Product, uint](db),
    }
}

func (r *ProductRepository) FindByCategory(ctx context.Context, catID uint) ([]Product, error) {
    filter, err := data.NewFilter(data.LogicalAnd,
        data.Condition{Field: "category_id", Operator: data.OperatorEqual, Value: catID},
    )
    if err != nil {
        return nil, err
    }
    return r.FindWhere(ctx, filter)
}
```

## Opérations CRUD

```go
ctx := context.Background()

// Créer / mettre à jour (upsert par clé primaire)
product := &Product{Name: "Clavier", Price: 149.99, CategoryID: 1}
if err := repo.Save(ctx, product); err != nil {
    return err
}
// product.ID est maintenant rempli

// Trouver par ID
p, err := repo.FindByID(ctx, 42)
if errors.Is(err, data.ErrRecordNotFound) {
    // gérer introuvable
}

// Trouver tout (limité à 1000 par défaut)
products, err := repo.FindAll(ctx)

// Supprimer
if err := repo.Delete(ctx, 42); err != nil {
    return err
}
```

## Filtrage

```go
import "github.com/enokdev/helix/data"

// Filtre simple
filter, err := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "price",       Operator: data.OperatorGreaterThan, Value: 50.0},
    data.Condition{Field: "category_id", Operator: data.OperatorEqual,       Value: 1},
)

products, err := repo.FindWhere(ctx, filter)
```

### Logique OR

```go
filter, err := data.NewFilter(data.LogicalOr,
    data.Condition{Field: "status", Operator: data.OperatorEqual, Value: "active"},
    data.Condition{Field: "status", Operator: data.OperatorEqual, Value: "featured"},
)
```

### Référence complète des opérateurs

| Opérateur | Constante | Équivalent SQL |
|-----------|-----------|----------------|
| Égal | `data.OperatorEqual` | `= ?` |
| Différent | `data.OperatorNotEqual` | `!= ?` |
| Supérieur | `data.OperatorGreaterThan` | `> ?` |
| Supérieur ou égal | `data.OperatorGreaterThanOrEqual` | `>= ?` |
| Inférieur | `data.OperatorLessThan` | `< ?` |
| Inférieur ou égal | `data.OperatorLessThanOrEqual` | `<= ?` |
| Contient | `data.OperatorContains` | `LIKE %?%` |
| Ne contient pas | `data.OperatorNotContains` | `NOT LIKE %?%` |
| Dans | `data.OperatorIn` | `IN (?)` |
| Pas dans | `data.OperatorNotIn` | `NOT IN (?)` |
| Est null | `data.OperatorIsNull` | `IS NULL` |
| N'est pas null | `data.OperatorIsNotNull` | `IS NOT NULL` |
| Entre | `data.OperatorBetween` | `BETWEEN ? AND ?` |

```go
// Entre : prix entre 10 et 100
filter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "price", Operator: data.OperatorBetween, Value: [2]float64{10, 100}},
)

// Pas dans : exclure certaines catégories
filter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "category_id", Operator: data.OperatorNotIn, Value: []uint{5, 7, 12}},
)
```

## Pagination

```go
page, err := repo.Paginate(ctx, 1, 20)  // page 1, 20 éléments par page

fmt.Println(page.Items)    // []Product
fmt.Println(page.Total)    // nombre total d'enregistrements
fmt.Println(page.Page)     // page courante
fmt.Println(page.PageSize) // 20
```

Les pages commencent à **1**. `ErrInvalidPagination` est retourné pour page < 1 ou size < 1.

## Connexion PostgreSQL et MySQL

GORM supporte plusieurs drivers. Passez le DSN approprié à `database.url` :

```yaml
# SQLite
database:
  url: "app.db"

# PostgreSQL
database:
  url: "postgres://user:password@localhost:5432/mydb?sslmode=disable"

# MySQL
database:
  url: "user:password@tcp(localhost:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"
```

## Transactions

### TransactionManager

```go
type ProductService struct {
    helix.Service
    Repo    *ProductRepository             `inject:"true"`
    TxMgr   *datagorm.TransactionManager  `inject:"true"`
}

func (s *ProductService) TransferStock(ctx context.Context, fromID, toID uint, qty int) error {
    return s.TxMgr.WithinTransaction(ctx, func(txCtx context.Context, tx data.Transaction[*gorm.DB]) error {
        txRepo := s.Repo.WithTransaction(tx)

        from, err := txRepo.FindByID(txCtx, fromID)
        if err != nil {
            return err
        }
        to, err := txRepo.FindByID(txCtx, toID)
        if err != nil {
            return err
        }

        from.Stock -= qty
        to.Stock += qty

        if err := txRepo.Save(txCtx, from); err != nil {
            return err
        }
        return txRepo.Save(txCtx, to)
        // commit sur return nil, rollback sur erreur
    })
}
```

## Connexion à la base de données

### SQLite (auto-configuré)

```yaml
# config/application.yaml
database:
  url: "app.db"
```

### Configuration du pool de connexions

```yaml
database:
  pool:
    max-open: 25
    max-idle: 5
```

## Limite de sécurité

`FindAll` et `FindWhere` sont limités à **1000 enregistrements** par défaut pour éviter les scans de table complets accidentels. Supprimez la limite pour le traitement en lot :

```go
repo := datagorm.NewRepository[Product, uint](db, datagorm.WithoutLimit())
```

## AutoMigrate vs migrations SQL

| Approche | Quand utiliser |
|----------|---------------|
| `AutoMigrate` | Développement, prototypes, applications SQLite |
| Fichiers de migration SQL | PostgreSQL/MySQL en production — les changements de schéma sont explicites et réversibles |

```bash
# Créer une migration SQL :
helix db migrate create add-stock-column
helix db migrate up
```

## Types d'erreurs

| Erreur | Signification |
|--------|--------------|
| `data.ErrRecordNotFound` | Aucun enregistrement ne correspond à la requête |
| `data.ErrDuplicateKey` | Violation de contrainte d'unicité |
| `data.ErrInvalidPagination` | Page ou taille hors limites |
| `data.ErrInvalidFilter` | Condition de filtre malformée |

### Gérer des erreurs spécifiques

```go
product, err := repo.FindByID(ctx, id)
switch {
case errors.Is(err, data.ErrRecordNotFound):
    return nil, web.NewRequestError(http.StatusNotFound, "produit introuvable")
case errors.Is(err, data.ErrDuplicateKey):
    return nil, web.NewRequestError(http.StatusConflict, "ce produit existe déjà")
case err != nil:
    return nil, err  // inattendu — devient 500
}
```
