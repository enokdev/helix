//go:build integration

package gorm_test

import (
	"context"
	"os"
	"testing"

	"github.com/enokdev/helix/data"
	datagorm "github.com/enokdev/helix/data/gorm"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	gormlib "gorm.io/gorm"
)

type dialectTarget struct {
	name string
	open func() gormlib.Dialector
}

func TestRepositoryExternalDialects(t *testing.T) {
	targets := externalDialectTargets()
	if len(targets) == 0 {
		t.Skip("set HELIX_GORM_POSTGRES_DSN or HELIX_GORM_MYSQL_DSN to run external dialect integration tests")
	}

	for _, target := range targets {
		target := target
		t.Run(target.name, func(t *testing.T) {
			ctx := context.Background()
			db := openDialectDB(t, target)
			repo := datagorm.NewRepository[integrationUser, int](db)

			nick := "countess"
			seedUsers(t, repo,
				integrationUser{Email: "ada@example.test", Name: "Ada Lovelace", Age: 36, Nickname: &nick},
				integrationUser{Email: "grace@example.test", Name: "Grace Hopper", Age: 85},
				integrationUser{Email: "alan@example.test", Name: "Alan Turing", Age: 41},
			)

			got, err := repo.FindWhere(ctx, mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Age", Operator: data.OperatorGreaterThanOrEqual, Value: 41},
				data.Condition{Field: "Email", Operator: data.OperatorIn, Value: []string{"grace@example.test", "alan@example.test"}},
			))
			if err != nil {
				t.Fatalf("FindWhere portable filters returned error: %v", err)
			}
			assertNames(t, got, []string{"Alan Turing", "Grace Hopper"})

			nullNames, err := repo.FindWhere(ctx, data.Filter{
				Logic: data.LogicalAnd,
				Conditions: []data.Condition{
					{Field: "Nickname", Operator: data.OperatorIsNull},
				},
			})
			if err != nil {
				t.Fatalf("FindWhere null filter returned error: %v", err)
			}
			assertNames(t, nullNames, []string{"Alan Turing", "Grace Hopper"})

			page, err := repo.Paginate(ctx, 2, 2)
			if err != nil {
				t.Fatalf("Paginate returned error: %v", err)
			}
			if page.Total != 3 || page.Page != 2 || page.PageSize != 2 || len(page.Items) != 1 {
				t.Fatalf("Paginate = total:%d page:%d size:%d items:%d, want 3/2/2/1", page.Total, page.Page, page.PageSize, len(page.Items))
			}

			manager := datagorm.NewTransactionManager(db)
			err = manager.WithinTransaction(ctx, func(txCtx context.Context, _ data.Transaction[*gormlib.DB]) error {
				return repo.Save(txCtx, &integrationUser{Email: "rollback@example.test", Name: "Rollback User"})
			})
			if err != nil {
				t.Fatalf("WithinTransaction commit returned error: %v", err)
			}
			assertEmailCount(t, repo, "rollback@example.test", 1)

			rollbackErr := manager.WithinTransaction(ctx, func(txCtx context.Context, _ data.Transaction[*gormlib.DB]) error {
				if err := repo.Save(txCtx, &integrationUser{Email: "discard@example.test", Name: "Discard User"}); err != nil {
					return err
				}
				return errDialectRollback
			})
			if rollbackErr == nil {
				t.Fatal("WithinTransaction rollback returned nil error")
			}
			assertEmailCount(t, repo, "discard@example.test", 0)
		})
	}
}

var errDialectRollback = assertRollbackError{}

type assertRollbackError struct{}

func (assertRollbackError) Error() string {
	return "force rollback"
}

func externalDialectTargets() []dialectTarget {
	var targets []dialectTarget
	if dsn := os.Getenv("HELIX_GORM_POSTGRES_DSN"); dsn != "" {
		targets = append(targets, dialectTarget{
			name: "postgres",
			open: func() gormlib.Dialector {
				return postgres.Open(dsn)
			},
		})
	}
	if dsn := os.Getenv("HELIX_GORM_MYSQL_DSN"); dsn != "" {
		targets = append(targets, dialectTarget{
			name: "mysql",
			open: func() gormlib.Dialector {
				return mysql.Open(dsn)
			},
		})
	}
	return targets
}

func openDialectDB(t *testing.T, target dialectTarget) *gormlib.DB {
	t.Helper()

	db, err := gormlib.Open(target.open(), &gormlib.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open %s db: %v", target.name, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get %s sql db: %v", target.name, err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.Migrator().DropTable(&integrationUser{}); err != nil {
		t.Fatalf("drop %s integration table: %v", target.name, err)
	}
	if err := db.AutoMigrate(&integrationUser{}); err != nil {
		t.Fatalf("auto migrate %s integration table: %v", target.name, err)
	}
	return db
}
