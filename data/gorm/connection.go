package gorm

import (
	"errors"
	"fmt"
	"reflect"

	"gorm.io/driver/sqlite"
	gormlib "gorm.io/gorm"
)

var (
	errEmptyDSN        = errors.New("empty DSN")
	errNilDB           = errors.New("nil database")
	errNegativePoolVal = errors.New("negative pool value")
	errNilModel        = errors.New("nil model")
)

// DB wraps a GORM database connection, hiding gorm.io types from other packages.
type DB struct {
	db *gormlib.DB
	tm *TransactionManager
}

// ConnectionPoolConfig holds database/sql connection-pool settings.
type ConnectionPoolConfig struct {
	MaxOpenConns int
	MaxIdleConns int
}

// OpenSQLite opens a SQLite database at dsn using GORM with TranslateError enabled.
func OpenSQLite(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("data/gorm: open sqlite: %w", errEmptyDSN)
	}
	gdb, err := gormlib.Open(sqlite.Open(dsn), &gormlib.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("data/gorm: open sqlite: %w", err)
	}
	return &DB{db: gdb, tm: NewTransactionManager(gdb)}, nil
}

// Components returns the components to register in the DI container:
// the internal *gorm.DB, *TransactionManager, and *DB wrapper.
func (d *DB) Components() []any {
	return []any{d.db, d.tm, d}
}

// ConfigurePool applies database/sql pool settings.
// Zero values are treated as no-op; negative values return an error.
func (d *DB) ConfigurePool(cfg ConnectionPoolConfig) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("data/gorm: configure pool: %w", errNilDB)
	}
	if cfg.MaxOpenConns < 0 || cfg.MaxIdleConns < 0 {
		return fmt.Errorf("data/gorm: configure pool: %w", errNegativePoolVal)
	}
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("data/gorm: configure pool: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	return nil
}

// Ping verifies the database connection is alive.
func (d *DB) Ping() error {
	if d == nil || d.db == nil {
		return fmt.Errorf("data/gorm: ping: %w", errNilDB)
	}
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("data/gorm: ping: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("data/gorm: ping: %w", err)
	}
	return nil
}

// Close releases all database resources.
func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("data/gorm: close: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("data/gorm: close: %w", err)
	}
	return nil
}

// AutoMigrate runs GORM AutoMigrate for the provided models.
func (d *DB) AutoMigrate(models ...any) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("data/gorm: auto migrate: %w", errNilDB)
	}
	for _, m := range models {
		if m == nil {
			return fmt.Errorf("data/gorm: auto migrate: %w", errNilModel)
		}
		rv := reflect.ValueOf(m)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return fmt.Errorf("data/gorm: auto migrate: %w", errNilModel)
		}
	}
	if err := d.db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("data/gorm: auto migrate: %w", err)
	}
	return nil
}

// HasTable reports whether the database has a table for model.
func (d *DB) HasTable(model any) bool {
	if d == nil || d.db == nil || model == nil {
		return false
	}
	return d.db.Migrator().HasTable(model)
}
