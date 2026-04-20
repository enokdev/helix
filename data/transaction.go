package data

import (
	"context"
	"errors"
	"reflect"
)

var (
	errInvalidTransactionContext = errors.New("data: invalid transaction context")
	errInvalidTransaction        = errors.New("data: invalid transaction")
)

// Transaction wraps an adapter-specific transaction without exposing the ORM.
// Implementations must not return nil from Unwrap.
type Transaction[TX any] interface {
	Unwrap() TX
}

// TransactionManager executes callbacks inside an adapter-specific transaction.
// Implementations must propagate an existing transaction from ctx when present.
type TransactionManager[TX any] interface {
	WithinTransaction(ctx context.Context, fn func(context.Context, Transaction[TX]) error) error
}

type transactionContextKey struct{}

// ContextWithTransaction returns a child context carrying tx for propagation.
// The transaction is keyed by TX, so different adapter transaction types do not collide.
func ContextWithTransaction[TX any](ctx context.Context, tx Transaction[TX]) (context.Context, error) {
	if ctx == nil {
		return nil, errInvalidTransactionContext
	}
	if tx == nil || isNilTransaction(tx) {
		return nil, errInvalidTransaction
	}

	transactions := cloneTransactions(ctx)
	transactions[transactionType[TX]()] = tx
	return context.WithValue(ctx, transactionContextKey{}, transactions), nil
}

// TransactionFromContext returns the active transaction for TX, when one exists.
func TransactionFromContext[TX any](ctx context.Context) (Transaction[TX], bool) {
	if ctx == nil {
		return nil, false
	}
	transactions, ok := ctx.Value(transactionContextKey{}).(map[reflect.Type]any)
	if !ok {
		return nil, false
	}
	tx, ok := transactions[transactionType[TX]()].(Transaction[TX])
	return tx, ok
}

func cloneTransactions(ctx context.Context) map[reflect.Type]any {
	existing, ok := ctx.Value(transactionContextKey{}).(map[reflect.Type]any)
	if !ok {
		return make(map[reflect.Type]any)
	}
	transactions := make(map[reflect.Type]any, len(existing)+1)
	for txType, tx := range existing {
		transactions[txType] = tx
	}
	return transactions
}

func transactionType[TX any]() reflect.Type {
	var zero TX
	if txType := reflect.TypeOf(zero); txType != nil {
		return txType
	}
	return reflect.TypeOf((*TX)(nil)).Elem()
}

func isNilTransaction[TX any](tx Transaction[TX]) bool {
	value := reflect.ValueOf(tx)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		// For struct-valued implementations, inspect Unwrap() to detect a wrapped nil.
		inner := reflect.ValueOf(tx.Unwrap())
		switch inner.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			return inner.IsNil()
		}
		return false
	}
}
