//go:build integration

package codegen

import (
	"context"
	"os/exec"
	"testing"
)

func TestGeneratedRepositoryQueriesSQLite(t *testing.T) {
	dir := t.TempDir()
	repoRoot := repoRoot(t)
	writeFixture(t, dir, "go.mod", "module example.test/generatedintegration\n\ngo 1.21.0\n\nrequire github.com/enokdev/helix v0.0.0\n\nreplace github.com/enokdev/helix => "+repoRoot+"\n")
	writeFixture(t, dir, "repository.go", validRepositorySource())
	writeFixture(t, dir, "repository_test.go", generatedRepositoryIntegrationSource())

	if _, err := NewGenerator(dir).Generate(context.Background()); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, output)
	}

	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated integration package failed: %v\n%s", err, output)
	}
}

func TestGeneratedTransactionalWrapperSQLite(t *testing.T) {
	dir := t.TempDir()
	repoRoot := repoRoot(t)
	writeFixture(t, dir, "go.mod", "module example.test/generatedtransactional\n\ngo 1.21.0\n\nrequire github.com/enokdev/helix v0.0.0\n\nreplace github.com/enokdev/helix => "+repoRoot+"\n")
	writeFixture(t, dir, "service.go", transactionalIntegrationSource())
	writeFixture(t, dir, "service_test.go", transactionalIntegrationTestSource())

	if _, err := NewGenerator(dir).Generate(context.Background()); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, output)
	}

	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated transactional package failed: %v\n%s", err, output)
	}
}

func generatedRepositoryIntegrationSource() string {
	return `package generated

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enokdev/helix/data"
	"gorm.io/driver/sqlite"
	gormlib "gorm.io/gorm"
)

func TestGeneratedRepositoryQueries(t *testing.T) {
	ctx := context.Background()
	db, err := gormlib.Open(sqlite.Open("file:generated_query?mode=memory&cache=shared"), &gormlib.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewUserRepository(db)
	users := []User{
		{Email: "ada@example.test", Name: "Ada_% Lovelace", Age: 36, CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{Email: "grace@example.test", Name: "Grace Hopper", Age: 85, CreatedAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
		{Email: "alan@example.test", Name: "Alan Turing", Age: 41, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for i := range users {
		if err := repo.Save(ctx, &users[i]); err != nil {
			t.Fatalf("save user: %v", err)
		}
	}

	ada, err := repo.FindByEmail(ctx, "ada@example.test")
	if err != nil {
		t.Fatalf("FindByEmail returned error: %v", err)
	}
	if ada.Name != "Ada_% Lovelace" {
		t.Fatalf("FindByEmail name = %q", ada.Name)
	}
	if _, err := repo.FindByEmail(ctx, "missing@example.test"); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("FindByEmail missing err = %v, want ErrRecordNotFound", err)
	}

	contains, err := repo.FindByNameContaining(ctx, "_%")
	if err != nil {
		t.Fatalf("FindByNameContaining returned error: %v", err)
	}
	if len(contains) != 1 || contains[0].Email != "ada@example.test" {
		t.Fatalf("FindByNameContaining returned %#v", contains)
	}

	older, err := repo.FindByAgeGreaterThan(ctx, 40)
	if err != nil {
		t.Fatalf("FindByAgeGreaterThan returned error: %v", err)
	}
	if len(older) != 2 {
		t.Fatalf("FindByAgeGreaterThan returned %d rows, want 2", len(older))
	}

	grace, err := repo.FindByEmailAndAge(ctx, "grace@example.test", 85)
	if err != nil {
		t.Fatalf("FindByEmailAndAge returned error: %v", err)
	}
	if grace.Name != "Grace Hopper" {
		t.Fatalf("FindByEmailAndAge name = %q", grace.Name)
	}

	ordered, err := repo.FindAllOrderByCreatedAtDesc(ctx)
	if err != nil {
		t.Fatalf("FindAllOrderByCreatedAtDesc returned error: %v", err)
	}
	if len(ordered) != 3 || ordered[0].Email != "grace@example.test" || ordered[2].Email != "alan@example.test" {
		t.Fatalf("FindAllOrderByCreatedAtDesc returned %#v", ordered)
	}
}
`
}

func transactionalIntegrationSource() string {
	return `package generated

import (
	"context"
	"errors"

	"github.com/enokdev/helix/data"
	datagorm "github.com/enokdev/helix/data/gorm"
	gormlib "gorm.io/gorm"
)

var errCreateFailed = errors.New("create failed")

type User struct {
	ID    int
	Email string
}

type UserWriter interface {
	CreateUser(ctx context.Context, email string) error
	CreateThenFail(ctx context.Context, email string) error
	CreateOuterThenFail(ctx context.Context, outerEmail, innerEmail string) error
	BuildUser(ctx context.Context, email string) (*User, error)
}

type UserService struct {
	repo data.Repository[User, int, *gormlib.DB]
	self UserWriter
}

func NewUserService(db *gormlib.DB) *UserService {
	service := &UserService{repo: datagorm.NewRepository[User, int](db)}
	service.self = service
	return service
}

func (s *UserService) UseSelf(self UserWriter) {
	s.self = self
}

//helix:transactional
func (s *UserService) CreateUser(ctx context.Context, email string) error {
	return s.repo.Save(ctx, &User{Email: email})
}

//helix:transactional
func (s *UserService) CreateThenFail(ctx context.Context, email string) error {
	if err := s.repo.Save(ctx, &User{Email: email}); err != nil {
		return err
	}
	return errCreateFailed
}

//helix:transactional
func (s *UserService) CreateOuterThenFail(ctx context.Context, outerEmail, innerEmail string) error {
	if err := s.repo.Save(ctx, &User{Email: outerEmail}); err != nil {
		return err
	}
	if err := s.self.CreateUser(ctx, innerEmail); err != nil {
		return err
	}
	return errCreateFailed
}

//helix:transactional
func (s *UserService) BuildUser(ctx context.Context, email string) (*User, error) {
	user := &User{Email: email}
	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
`
}

func transactionalIntegrationTestSource() string {
	return `package generated

import (
	"context"
	"errors"
	"testing"

	datagorm "github.com/enokdev/helix/data/gorm"
	"gorm.io/driver/sqlite"
	gormlib "gorm.io/gorm"
)

func TestGeneratedTransactionalWrapper(t *testing.T) {
	ctx := context.Background()
	db, err := gormlib.Open(sqlite.Open("file:generated_transactional?mode=memory&cache=shared"), &gormlib.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewUserService(db)
	wrapped := NewUserServiceTransactional(service, datagorm.NewTransactionManager(db))
	service.UseSelf(wrapped)

	var writer UserWriter = wrapped
	if err := writer.CreateUser(ctx, "commit@example.test"); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	assertUserCount(t, db, "commit@example.test", 1)

	if err := writer.CreateThenFail(ctx, "rollback@example.test"); !errors.Is(err, errCreateFailed) {
		t.Fatalf("CreateThenFail error = %v, want errCreateFailed", err)
	}
	assertUserCount(t, db, "rollback@example.test", 0)

	if err := writer.CreateOuterThenFail(ctx, "outer@example.test", "inner@example.test"); !errors.Is(err, errCreateFailed) {
		t.Fatalf("CreateOuterThenFail error = %v, want errCreateFailed", err)
	}
	assertUserCount(t, db, "outer@example.test", 0)
	assertUserCount(t, db, "inner@example.test", 0)

	user, err := writer.BuildUser(ctx, "built@example.test")
	if err != nil {
		t.Fatalf("BuildUser returned error: %v", err)
	}
	if user == nil || user.ID == 0 {
		t.Fatalf("BuildUser returned %#v, want persisted user", user)
	}
	assertUserCount(t, db, "built@example.test", 1)
}

func assertUserCount(t *testing.T, db *gormlib.DB, email string, want int64) {
	t.Helper()

	var count int64
	if err := db.Model(&User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		t.Fatalf("count %q: %v", email, err)
	}
	if count != want {
		t.Fatalf("count %q = %d, want %d", email, count, want)
	}
}
`
}
