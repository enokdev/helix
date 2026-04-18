package gorm

import gormlib "gorm.io/gorm"

// Transaction wraps a GORM transaction handle for data.Repository.WithTransaction.
type Transaction struct {
	db *gormlib.DB
}

// NewTransaction wraps db as a data.Transaction-compatible GORM transaction.
func NewTransaction(db *gormlib.DB) Transaction {
	return Transaction{db: db}
}

// Unwrap returns the wrapped GORM transaction handle.
func (t Transaction) Unwrap() *gormlib.DB {
	return t.db
}
