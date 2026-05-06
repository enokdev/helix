package gorm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/enokdev/helix/data"
	gormlib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errInvalidRepository  = errors.New("invalid repository")
	errInvalidDB          = errors.New("invalid database handle")
	errInvalidEntity      = errors.New("invalid entity")
	errInvalidContext     = errors.New("invalid context")
	errInvalidTransaction = errors.New("invalid transaction")
	errInvalidPage        = data.ErrInvalidPagination
	errInvalidTotal       = errors.New("invalid total")
)

const defaultFindAllLimit = 1000

// Compile-time check that *Repository[T, ID] implements data.Repository[T, ID, *gormlib.DB].
var _ data.Repository[any, any, *gormlib.DB] = (*Repository[any, any])(nil)

// Option customizes a GORM repository.
type Option func(*repositoryOptions)

type repositoryOptions struct {
	findAllLimit int
}

// Repository implements data.Repository using a GORM database handle.
type Repository[T any, ID any] struct {
	db      *gormlib.DB
	err     error
	options repositoryOptions
}

// WithoutLimit disables the default FindAll safety limit for callers that
// explicitly need to load every record.
func WithoutLimit() Option {
	return func(opts *repositoryOptions) {
		opts.findAllLimit = 0
	}
}

// NewRepository creates a GORM-backed repository.
// If db is nil, the returned repository is invalid: every method call will
// return an error wrapping errInvalidDB without panicking.
func NewRepository[T, ID any](db *gormlib.DB, opts ...Option) *Repository[T, ID] {
	options := defaultRepositoryOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	if db == nil {
		return &Repository[T, ID]{err: errInvalidDB, options: options}
	}
	return &Repository[T, ID]{db: db, options: options}
}

// FindAll returns all records for T.
func (r *Repository[T, ID]) FindAll(ctx context.Context) ([]T, error) {
	db, err := r.database(ctx, "find all")
	if err != nil {
		return nil, err
	}

	var items []T
	limit := r.options.findAllLimit
	query := db
	if limit > 0 {
		// Fetch one extra row to detect truncation without a COUNT query.
		// A table with exactly `limit` rows will return limit items (not limit+1),
		// so no false-positive warning is emitted.
		query = query.Limit(limit + 1)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, wrapError("find all", err)
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
		slog.WarnContext(ctx, "data/gorm: find all limit applied",
			"action", "find all",
			"limit", limit,
			"repository", fmt.Sprintf("%T", r),
		)
	}
	return items, nil
}

// FindByID returns the record matching id.
func (r *Repository[T, ID]) FindByID(ctx context.Context, id ID) (*T, error) {
	db, err := r.database(ctx, "find by id")
	if err != nil {
		return nil, err
	}

	var item T
	if err := db.First(&item, id).Error; err != nil {
		return nil, wrapError("find by id", err)
	}
	return &item, nil
}

// FindWhere returns records matching filter.
func (r *Repository[T, ID]) FindWhere(ctx context.Context, filter data.Filter) ([]T, error) {
	db, err := r.database(ctx, "find where")
	if err != nil {
		return nil, err
	}
	db, err = r.applyFilter(db, filter)
	if err != nil {
		return nil, wrapError("find where", err)
	}

	var items []T
	if err := db.Find(&items).Error; err != nil {
		return nil, wrapError("find where", err)
	}
	return items, nil
}

// Save creates or updates entity using GORM's traditional upsert semantics.
func (r *Repository[T, ID]) Save(ctx context.Context, entity *T) error {
	db, err := r.database(ctx, "save")
	if err != nil {
		return err
	}
	if entity == nil {
		return wrapError("save", errInvalidEntity)
	}

	if err := db.Save(entity).Error; err != nil {
		return wrapError("save", err)
	}
	return nil
}

// Delete removes the record matching id.
func (r *Repository[T, ID]) Delete(ctx context.Context, id ID) error {
	db, err := r.database(ctx, "delete")
	if err != nil {
		return err
	}

	result := db.Delete(new(T), id)
	if result.Error != nil {
		return wrapError("delete", result.Error)
	}
	if result.RowsAffected == 0 {
		return wrapError("delete", data.ErrRecordNotFound)
	}
	return nil
}

// Paginate returns one page of records and the total count.
func (r *Repository[T, ID]) Paginate(ctx context.Context, page, size int) (data.Page[T], error) {
	db, err := r.database(ctx, "paginate")
	if err != nil {
		return data.Page[T]{}, err
	}
	if page < 1 || size < 1 {
		return data.Page[T]{}, wrapError("paginate", errInvalidPage)
	}
	if size > 1000 {
		size = 1000
	}
	// Avoid overflow: (page-1)*size must not exceed maxInt.
	// Check (page-1) > maxInt()/size instead of page > (maxInt()/size)+1
	// because the latter overflows to minInt when size == 1.
	if page-1 > maxInt()/size {
		return data.Page[T]{}, wrapError("paginate", errInvalidPage)
	}

	var total int64
	if err := db.Model(new(T)).Count(&total).Error; err != nil {
		return data.Page[T]{}, wrapError("paginate", err)
	}
	if total > int64(maxInt()) {
		return data.Page[T]{}, wrapError("paginate", errInvalidTotal)
	}

	var items []T
	offset := (page - 1) * size
	if err := db.Limit(size).Offset(offset).Find(&items).Error; err != nil {
		return data.Page[T]{}, wrapError("paginate", err)
	}

	return data.Page[T]{
		Items:    items,
		Total:    int(total),
		Page:     page,
		PageSize: size,
	}, nil
}

// WithTransaction returns a new Repository bound to tx.
func (r *Repository[T, ID]) WithTransaction(tx data.Transaction[*gormlib.DB]) data.Repository[T, ID, *gormlib.DB] {
	if tx == nil || isNilValue(tx) {
		return &Repository[T, ID]{err: errInvalidDB}
	}
	db := tx.Unwrap()
	if db == nil {
		return &Repository[T, ID]{err: errInvalidDB}
	}
	options := defaultRepositoryOptions()
	if r != nil {
		options = r.options
	}
	return &Repository[T, ID]{db: db, options: options}
}

func (r *Repository[T, ID]) database(ctx context.Context, action string) (*gormlib.DB, error) {
	if r == nil {
		return nil, wrapError(action, errInvalidRepository)
	}
	if ctx == nil {
		return nil, wrapError(action, errInvalidContext)
	}
	if r.err != nil {
		return nil, wrapError(action, r.err)
	}
	if tx, ok := data.TransactionFromContext[*gormlib.DB](ctx); ok {
		db := tx.Unwrap()
		if db == nil {
			return nil, wrapError(action, errInvalidTransaction)
		}
		return db.WithContext(ctx), nil
	}
	if r.db == nil {
		return nil, wrapError(action, errInvalidDB)
	}
	return r.db.WithContext(ctx), nil
}

// Database validates db and binds ctx for generated GORM queries.
func Database(ctx context.Context, db *gormlib.DB, action string) (*gormlib.DB, error) {
	if ctx == nil {
		return nil, wrapError(action, errInvalidContext)
	}
	if tx, ok := data.TransactionFromContext[*gormlib.DB](ctx); ok {
		txDB := tx.Unwrap()
		if txDB == nil {
			return nil, wrapError(action, errInvalidTransaction)
		}
		return txDB.WithContext(ctx), nil
	}
	if db == nil {
		return nil, wrapError(action, errInvalidDB)
	}
	return db.WithContext(ctx), nil
}

func (r *Repository[T, ID]) applyFilter(db *gormlib.DB, filter data.Filter) (*gormlib.DB, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if len(filter.Conditions) == 0 {
		return db, nil
	}

	expressions := make([]clause.Expression, 0, len(filter.Conditions))
	for _, condition := range filter.Conditions {
		expression, err := r.expressionFor(db, condition)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, expression)
	}

	if filter.Logic == data.LogicalOr {
		query := db.Where(expressions[0])
		for _, expression := range expressions[1:] {
			query = query.Or(expression)
		}
		return query, nil
	}

	query := db
	for _, expression := range expressions {
		query = query.Where(expression)
	}
	return query, nil
}

func (r *Repository[T, ID]) expressionFor(db *gormlib.DB, condition data.Condition) (clause.Expression, error) {
	column, err := r.columnFor(db, condition.Field)
	if err != nil {
		return nil, err
	}

	switch condition.Operator {
	case data.OperatorEqual:
		return clause.Eq{Column: column, Value: condition.Value}, nil
	case data.OperatorNotEqual:
		return clause.Neq{Column: column, Value: condition.Value}, nil
	case data.OperatorGreaterThan:
		return clause.Gt{Column: column, Value: condition.Value}, nil
	case data.OperatorGreaterThanOrEqual:
		return clause.Gte{Column: column, Value: condition.Value}, nil
	case data.OperatorLessThan:
		return clause.Lt{Column: column, Value: condition.Value}, nil
	case data.OperatorLessThanOrEqual:
		return clause.Lte{Column: column, Value: condition.Value}, nil
	case data.OperatorContains:
		value, ok := condition.Value.(string)
		if !ok {
			return nil, data.ErrInvalidFilter
		}
		return clause.Expr{
			SQL:  "? LIKE ? ESCAPE '\\'",
			Vars: []any{column, "%" + escapeLike(value) + "%"},
		}, nil
	case data.OperatorIn:
		values := valuesForIN(condition.Value)
		if len(values) == 0 {
			return nil, data.ErrInvalidFilter
		}
		return clause.IN{Column: column, Values: values}, nil
	case data.OperatorIsNull:
		return clause.Eq{Column: column, Value: nil}, nil
	case data.OperatorIsNotNull:
		return clause.Neq{Column: column, Value: nil}, nil
	default:
		return nil, data.ErrInvalidFilter
	}
}

// ColumnFor resolves field to a validated GORM column for T.
func ColumnFor[T any](db *gormlib.DB, field string) (clause.Column, error) {
	if err := data.ValidateFieldName(field); err != nil {
		return clause.Column{}, err
	}

	stmt := &gormlib.Statement{DB: db}
	if err := stmt.Parse(new(T)); err != nil {
		return clause.Column{}, err
	}
	if stmt.Schema == nil {
		return clause.Column{}, data.ErrInvalidFilter
	}

	schemaField := stmt.Schema.LookUpField(field)
	if schemaField == nil || schemaField.DBName == "" {
		return clause.Column{}, data.ErrInvalidFilter
	}
	return clause.Column{Name: schemaField.DBName}, nil
}

func (r *Repository[T, ID]) columnFor(db *gormlib.DB, field string) (clause.Column, error) {
	return ColumnFor[T](db, field)
}

// WrapError maps adapter-specific errors to public Helix data errors.
func WrapError(action string, err error) error {
	return wrapError(action, err)
}

func wrapError(action string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, gormlib.ErrRecordNotFound), errors.Is(err, data.ErrRecordNotFound):
		return fmt.Errorf("data/gorm: %s: %w", action, data.ErrRecordNotFound)
	case errors.Is(err, gormlib.ErrDuplicatedKey), errors.Is(err, data.ErrDuplicateKey):
		return fmt.Errorf("data/gorm: %s: %w", action, data.ErrDuplicateKey)
	case errors.Is(err, data.ErrInvalidFilter):
		return fmt.Errorf("data/gorm: %s: %w", action, err)
	default:
		return fmt.Errorf("data/gorm: %s: %w", action, err)
	}
}

func valuesForIN(value any) []any {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}

	values := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		values = append(values, rv.Index(i).Interface())
	}
	return values
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func defaultRepositoryOptions() repositoryOptions {
	return repositoryOptions{findAllLimit: defaultFindAllLimit}
}

// isNilValue reports whether v is a typed nil (e.g. (*T)(nil) wrapped in an interface).
// It prevents WithTransaction from panicking when the caller passes a nil concrete pointer
// through a non-nil interface value.
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
