package data

import "context"

// Repository defines the generic, ORM-neutral persistence contract.
// TX is the adapter-specific transaction type exposed via Transaction[TX].
type Repository[T any, ID any, TX any] interface {
	FindAll(ctx context.Context) ([]T, error)
	FindByID(ctx context.Context, id ID) (*T, error)
	FindWhere(ctx context.Context, filter Filter) ([]T, error)
	Save(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id ID) error
	Paginate(ctx context.Context, page, size int) (Page[T], error)
	// WithTransaction returns a new Repository bound to tx. tx must not be nil.
	WithTransaction(tx Transaction[TX]) Repository[T, ID, TX]
}
