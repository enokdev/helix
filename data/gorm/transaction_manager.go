package gorm

import (
	"context"

	"github.com/enokdev/helix/data"
	gormlib "gorm.io/gorm"
)

// Compile-time check that TransactionManager satisfies the ORM-neutral contract.
var _ data.TransactionManager[*gormlib.DB] = (*TransactionManager)(nil)

// TransactionManager executes callbacks inside GORM transactions.
type TransactionManager struct {
	db *gormlib.DB
}

// NewTransactionManager creates a transaction manager backed by db.
// A nil db is reported when WithinTransaction is called.
func NewTransactionManager(db *gormlib.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

// WithinTransaction runs fn inside a GORM transaction.
// If ctx already carries a GORM transaction, fn joins it instead of creating
// a nested GORM transaction or savepoint.
func (m *TransactionManager) WithinTransaction(ctx context.Context, fn func(context.Context, data.Transaction[*gormlib.DB]) error) error {
	if m == nil || m.db == nil {
		return wrapError("transaction", errInvalidDB)
	}
	if ctx == nil {
		return wrapError("transaction", errInvalidContext)
	}
	if fn == nil {
		return wrapError("transaction", errInvalidTransaction)
	}

	if tx, ok := data.TransactionFromContext[*gormlib.DB](ctx); ok {
		if tx.Unwrap() == nil {
			return wrapError("transaction", errInvalidTransaction)
		}
		return fn(ctx, tx)
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gormlib.DB) error {
		transaction := NewTransaction(tx)
		txCtx, err := data.ContextWithTransaction[*gormlib.DB](ctx, transaction)
		if err != nil {
			return wrapError("transaction", err)
		}
		return fn(txCtx, transaction)
	})
}
