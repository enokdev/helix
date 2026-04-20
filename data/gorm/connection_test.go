//go:build integration

package gorm_test

import (
	"testing"

	datagorm "github.com/enokdev/helix/data/gorm"
)

type connTestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null"`
}

func TestOpenSQLiteValidDSN(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite(\":memory:\") error = %v", err)
	}
	defer db.Close()
}

func TestOpenSQLiteEmptyDSN(t *testing.T) {
	_, err := datagorm.OpenSQLite("")
	if err == nil {
		t.Fatal("OpenSQLite(\"\") expected error, got nil")
	}
}

func TestConfigurePoolValidValues(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	defer db.Close()

	if err := db.ConfigurePool(datagorm.ConnectionPoolConfig{MaxOpenConns: 5, MaxIdleConns: 2}); err != nil {
		t.Fatalf("ConfigurePool valid error = %v", err)
	}
}

func TestConfigurePoolZeroIsNoOp(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	defer db.Close()

	if err := db.ConfigurePool(datagorm.ConnectionPoolConfig{}); err != nil {
		t.Fatalf("ConfigurePool zero values error = %v", err)
	}
}

func TestConfigurePoolNegativeValues(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	defer db.Close()

	if err := db.ConfigurePool(datagorm.ConnectionPoolConfig{MaxOpenConns: -1}); err == nil {
		t.Fatal("ConfigurePool negative MaxOpenConns expected error, got nil")
	}
	if err := db.ConfigurePool(datagorm.ConnectionPoolConfig{MaxIdleConns: -1}); err == nil {
		t.Fatal("ConfigurePool negative MaxIdleConns expected error, got nil")
	}
}

func TestPingAndClose(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestAutoMigrateAndHasTable(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	defer db.Close()

	if db.HasTable(&connTestModel{}) {
		t.Fatal("HasTable before AutoMigrate = true, want false")
	}

	if err := db.AutoMigrate(&connTestModel{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}

	if !db.HasTable(&connTestModel{}) {
		t.Fatal("HasTable after AutoMigrate = false, want true")
	}
}

func TestComponentsContainsDBAndTransactionManager(t *testing.T) {
	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	defer db.Close()

	comps := db.Components()
	if len(comps) < 2 {
		t.Fatalf("Components() returned %d components, want at least 2", len(comps))
	}

	var hasDB, hasTM bool
	for _, c := range comps {
		switch c.(type) {
		case *datagorm.DB:
			hasDB = true
		case *datagorm.TransactionManager:
			hasTM = true
		}
	}
	if !hasDB {
		t.Fatal("Components() does not contain *datagorm.DB")
	}
	if !hasTM {
		t.Fatal("Components() does not contain *datagorm.TransactionManager")
	}
}
