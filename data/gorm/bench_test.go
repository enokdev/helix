package gorm_test

import (
	"context"
	"fmt"
	"testing"

	datagorm "github.com/enokdev/helix/data/gorm"
	"gorm.io/driver/sqlite"
	gormlib "gorm.io/gorm"
)

type benchmarkUser struct {
	ID    int `gorm:"primaryKey"`
	Email string
	Name  string
}

func BenchmarkRepositoryFindByIDSQLite(b *testing.B) {
	ctx := context.Background()
	db, err := gormlib.Open(sqlite.Open(":memory:"), &gormlib.Config{})
	if err != nil {
		b.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&benchmarkUser{}); err != nil {
		b.Fatalf("AutoMigrate() error = %v", err)
	}
	users := make([]benchmarkUser, 100)
	for i := range users {
		users[i] = benchmarkUser{
			Email: fmt.Sprintf("user-%d@example.test", i+1),
			Name:  fmt.Sprintf("User %d", i+1),
		}
	}
	if err := db.Create(&users).Error; err != nil {
		b.Fatalf("seed users: %v", err)
	}
	repo := datagorm.NewRepository[benchmarkUser, int](db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := users[i%len(users)].ID
		if _, err := repo.FindByID(ctx, id); err != nil {
			b.Fatalf("FindByID(%d) error = %v", id, err)
		}
	}
}
