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

## Error Types

| Error | Meaning |
|-------|---------|
| `data.ErrRecordNotFound` | No record matched the query |
| `data.ErrDuplicateKey` | Unique constraint violation |
| `data.ErrInvalidPagination` | Page or size out of range |
| `data.ErrInvalidFilter` | Malformed filter condition |
