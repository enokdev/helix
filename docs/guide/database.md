# Database & Repository

Helix provides a generic repository pattern with a GORM adapter. It keeps your domain code decoupled from the ORM while giving you full access to GORM when needed.

## The Repository Interface

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

## GORM Adapter

### Define your model

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

### Create a repository

```go
import datagorm "github.com/enokdev/helix/data/gorm"

repo := datagorm.NewRepository[Product, uint](db)
```

That's it — you now have `FindAll`, `FindByID`, `FindWhere`, `Save`, `Delete`, `Paginate`.

### Custom repository

Embed the GORM repository in your domain repository to extend it:

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

## CRUD Operations

```go
ctx := context.Background()

// Create / update (upsert by primary key)
product := &Product{Name: "Keyboard", Price: 149.99, CategoryID: 1}
if err := repo.Save(ctx, product); err != nil {
    return err
}
// product.ID is now populated

// Find by ID
p, err := repo.FindByID(ctx, 42)
if errors.Is(err, data.ErrRecordNotFound) {
    // handle not found
}

// Find all (limited to 1000 by default)
products, err := repo.FindAll(ctx)

// Delete
if err := repo.Delete(ctx, 42); err != nil {
    return err
}
```

## Filtering

```go
import "github.com/enokdev/helix/data"

// Simple filter
filter, err := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "price",       Operator: data.OperatorGreaterThan, Value: 50.0},
    data.Condition{Field: "category_id", Operator: data.OperatorEqual,       Value: 1},
)

products, err := repo.FindWhere(ctx, filter)
```

### OR logic

```go
filter, err := data.NewFilter(data.LogicalOr,
    data.Condition{Field: "status", Operator: data.OperatorEqual, Value: "active"},
    data.Condition{Field: "status", Operator: data.OperatorEqual, Value: "featured"},
)
```

### Available operators

| Operator | SQL equivalent |
|----------|---------------|
| `OperatorEqual` | `= ?` |
| `OperatorNotEqual` | `!= ?` |
| `OperatorGreaterThan` | `> ?` |
| `OperatorGreaterThanOrEqual` | `>= ?` |
| `OperatorLessThan` | `< ?` |
| `OperatorLessThanOrEqual` | `<= ?` |
| `OperatorContains` | `LIKE %?%` |
| `OperatorIn` | `IN (?)` |
| `OperatorIsNull` | `IS NULL` |
| `OperatorIsNotNull` | `IS NOT NULL` |

## Pagination

```go
page, err := repo.Paginate(ctx, 1, 20)  // page 1, 20 items per page

fmt.Println(page.Items)    // []Product
fmt.Println(page.Total)    // total record count
fmt.Println(page.Page)     // current page
fmt.Println(page.PageSize) // 20
```

Pages start at **1**. `ErrInvalidPagination` is returned for page < 1 or size < 1.

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
        // commits on return nil, rolls back on error
    })
}
```

### Context propagation

The transaction is stored in the context, so you can pass it down call chains without changing signatures:

```go
tx, ok := data.TransactionFromContext[*gorm.DB](ctx)
```

## Database Connection

### SQLite (auto-configured)

```yaml
# config/application.yaml
database:
  url: "app.db"
```

Add the data starter and it connects automatically:

```go
helix.Run(helix.App{
    Starters: []starter.Entry{
        {Name: "data", Order: starter.OrderData,
         Starter: datastarter.New(loader,
             datastarter.WithAutoMigrateModels(&Product{}, &Category{}),
         )},
    },
})
```

### Manual connection

```go
db, err := datagorm.OpenSQLite("app.db")
if err != nil {
    return err
}

db.ConfigurePool(datagorm.ConnectionPoolConfig{
    MaxOpenConns: 25,
    MaxIdleConns: 5,
})

// Auto-migrate schemas
db.AutoMigrate(&Product{}, &Category{})

// Pass *gorm.DB to repositories
gormDB := db.Components()[0].(*gorm.DB)
repo := datagorm.NewRepository[Product, uint](gormDB)
```

### Connection pool config

```yaml
database:
  pool:
    max-open: 25
    max-idle: 5
```

## Safety Limit

`FindAll` and `FindWhere` are limited to **1000 records** by default to prevent accidental full-table scans. Remove the limit for batch processing:

```go
repo := datagorm.NewRepository[Product, uint](db, datagorm.WithoutLimit())
```

## PostgreSQL and MySQL connections

GORM supports multiple drivers. Pass the appropriate DSN to `database.url`:

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

Import the driver in your `main.go`:

```go
import (
    _ "github.com/lib/pq"           // PostgreSQL
    _ "github.com/go-sql-driver/mysql" // MySQL
)
```

Or use GORM's native driver packages:

```go
import (
    "gorm.io/driver/postgres"
    "gorm.io/driver/mysql"
    "gorm.io/driver/sqlite"
)
```

## Complete operator reference

| Operator | Constant | SQL equivalent |
|----------|----------|---------------|
| Equal | `data.OperatorEqual` | `= ?` |
| Not equal | `data.OperatorNotEqual` | `!= ?` |
| Greater than | `data.OperatorGreaterThan` | `> ?` |
| Greater or equal | `data.OperatorGreaterThanOrEqual` | `>= ?` |
| Less than | `data.OperatorLessThan` | `< ?` |
| Less or equal | `data.OperatorLessThanOrEqual` | `<= ?` |
| Contains (LIKE) | `data.OperatorContains` | `LIKE %?%` |
| Not contains | `data.OperatorNotContains` | `NOT LIKE %?%` |
| In | `data.OperatorIn` | `IN (?)` |
| Not in | `data.OperatorNotIn` | `NOT IN (?)` |
| Is null | `data.OperatorIsNull` | `IS NULL` |
| Is not null | `data.OperatorIsNotNull` | `IS NOT NULL` |
| Between | `data.OperatorBetween` | `BETWEEN ? AND ?` |

```go
// Between: price between 10 and 100
filter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "price", Operator: data.OperatorBetween, Value: [2]float64{10, 100}},
)

// Not in: exclude certain categories
filter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "category_id", Operator: data.OperatorNotIn, Value: []uint{5, 7, 12}},
)

// Is null: products with no description
filter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "description", Operator: data.OperatorIsNull},
)
```

## Complex (nested) filters

Combine AND and OR by nesting filters:

```go
// (price > 50 AND category_id = 1) OR (featured = true)
activeFilter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "price",       Operator: data.OperatorGreaterThan, Value: 50.0},
    data.Condition{Field: "category_id", Operator: data.OperatorEqual,       Value: 1},
)

featuredFilter, _ := data.NewFilter(data.LogicalAnd,
    data.Condition{Field: "featured", Operator: data.OperatorEqual, Value: true},
)

combined, _ := data.NewFilter(data.LogicalOr,
    activeFilter.AsCondition(),
    featuredFilter.AsCondition(),
)

products, err := repo.FindWhere(ctx, combined)
```

## Raw GORM access

When the repository abstraction is not enough, access the underlying `*gorm.DB` directly:

```go
type ProductRepository struct {
    helix.Repository
    *datagorm.Repository[Product, uint]
    db *gorm.DB  // stored separately for raw access
}

func NewProductRepository(db *datagorm.DB) *ProductRepository {
    gormDB := db.DB()  // unwrap *gorm.DB
    return &ProductRepository{
        Repository: datagorm.NewRepository[Product, uint](gormDB),
        db:         gormDB,
    }
}

// Raw query for complex aggregation:
func (r *ProductRepository) SalesByCategory(ctx context.Context) ([]CategorySales, error) {
    var results []CategorySales
    err := r.db.WithContext(ctx).
        Raw(`SELECT category_id, SUM(price * sold) as total
             FROM products
             GROUP BY category_id
             ORDER BY total DESC`).
        Scan(&results).Error
    return results, err
}

// Raw update for bulk operations:
func (r *ProductRepository) MarkOutOfStock(ctx context.Context, ids []uint) error {
    return r.db.WithContext(ctx).
        Model(&Product{}).
        Where("id IN ?", ids).
        Update("stock", 0).Error
}
```

## AutoMigrate vs SQL migrations

| Approach | When to use |
|----------|------------|
| `AutoMigrate` | Development, prototypes, SQLite apps |
| SQL migration files | Production PostgreSQL/MySQL — schema changes are explicit and reversible |

```go
// AutoMigrate (adds columns, creates tables — never drops):
db.AutoMigrate(&Product{}, &Category{}, &Order{})

// SQL migration (via helix CLI):
// helix db migrate create add-stock-column
// helix db migrate up
```

For production, always use `helix db migrate` — it tracks applied migrations in a `schema_migrations` table and supports rollback.

## Error Types

| Error | Meaning |
|-------|---------|
| `data.ErrRecordNotFound` | No record matched the query |
| `data.ErrDuplicateKey` | Unique constraint violation |
| `data.ErrInvalidPagination` | Page or size out of range |
| `data.ErrInvalidFilter` | Malformed filter condition |

### Handling specific errors

```go
product, err := repo.FindByID(ctx, id)
switch {
case errors.Is(err, data.ErrRecordNotFound):
    return nil, web.NewRequestError(http.StatusNotFound, "product not found")
case errors.Is(err, data.ErrDuplicateKey):
    return nil, web.NewRequestError(http.StatusConflict, "product already exists")
case err != nil:
    return nil, err  // unexpected — becomes 500
}
```
