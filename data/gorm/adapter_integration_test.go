//go:build integration

package gorm_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/enokdev/helix/data"
	datagorm "github.com/enokdev/helix/data/gorm"
	"gorm.io/driver/sqlite"
	gormlib "gorm.io/gorm"
)

type integrationUser struct {
	ID       int    `gorm:"primaryKey"`
	Email    string `gorm:"uniqueIndex"`
	Name     string
	Age      int
	Nickname *string
}

func TestRepositoryImplementsDataRepository(t *testing.T) {
	var _ data.Repository[integrationUser, int, *gormlib.DB] = (*datagorm.Repository[integrationUser, int])(nil)
}

func TestRepositoryCRUDAndErrorMapping(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := datagorm.NewRepository[integrationUser, int](db)

	ada := integrationUser{Email: "ada@example.test", Name: "Ada", Age: 36}
	if err := repo.Save(ctx, &ada); err != nil {
		t.Fatalf("Save create returned error: %v", err)
	}
	if ada.ID == 0 {
		t.Fatal("Save create did not populate the primary key")
	}

	duplicate := integrationUser{Email: "ada@example.test", Name: "Other Ada", Age: 37}
	if err := repo.Save(ctx, &duplicate); !errors.Is(err, data.ErrDuplicateKey) {
		t.Fatalf("Save duplicate error = %v, want ErrDuplicateKey", err)
	}

	ada.Name = "Ada Lovelace"
	if err := repo.Save(ctx, &ada); err != nil {
		t.Fatalf("Save update returned error: %v", err)
	}

	found, err := repo.FindByID(ctx, ada.ID)
	if err != nil {
		t.Fatalf("FindByID existing returned error: %v", err)
	}
	if found.Name != "Ada Lovelace" {
		t.Fatalf("FindByID returned name %q, want Ada Lovelace", found.Name)
	}

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("FindAll returned %d users, want 1", len(all))
	}

	if err := repo.Delete(ctx, ada.ID); err != nil {
		t.Fatalf("Delete existing returned error: %v", err)
	}
	if _, err := repo.FindByID(ctx, ada.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("FindByID deleted error = %v, want ErrRecordNotFound", err)
	}
	if err := repo.Delete(ctx, ada.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("Delete absent error = %v, want ErrRecordNotFound", err)
	}
}

func TestRepositoryRejectsNilInputs(t *testing.T) {
	ctx := context.Background()
	repo := datagorm.NewRepository[integrationUser, int](openIntegrationDB(t))

	if err := repo.Save(ctx, nil); err == nil {
		t.Fatal("Save nil entity returned nil error")
	}

	nilDBRepo := datagorm.NewRepository[integrationUser, int](nil)
	if _, err := nilDBRepo.FindAll(ctx); err == nil {
		t.Fatal("FindAll on nil db repository returned nil error")
	}
}

func TestRepositoryFindAllAppliesDefaultLimitAndWarns(t *testing.T) {
	ctx := context.Background()
	handler := &captureHandler{}
	setDefaultLoggerForTest(t, slog.New(handler))

	db := openIntegrationDB(t)
	truncateUsers(t, db)
	repo := datagorm.NewRepository[integrationUser, int](db)
	seedManyUsers(t, repo, 1005)

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1000 {
		t.Fatalf("FindAll returned %d users, want default limit 1000", len(all))
	}

	record, ok := handler.find("data/gorm: find all limit applied")
	if !ok {
		t.Fatal("FindAll did not log default limit warning")
	}
	if got := attrValue(record, "action"); got != "find all" {
		t.Fatalf("warning action attr = %v, want find all", got)
	}
	if got := attrValue(record, "limit"); got != int64(1000) {
		t.Fatalf("warning limit attr = %v, want 1000", got)
	}
	if got := attrValue(record, "repository"); got == "" {
		t.Fatal("warning repository attr is empty")
	}
}

func TestRepositoryFindAllDoesNotWarnAtExactLimitBoundary(t *testing.T) {
	ctx := context.Background()
	handler := &captureHandler{}
	setDefaultLoggerForTest(t, slog.New(handler))

	db := openIntegrationDB(t)
	truncateUsers(t, db)
	repo := datagorm.NewRepository[integrationUser, int](db)
	seedManyUsers(t, repo, 1000) // exactly at the default limit

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1000 {
		t.Fatalf("FindAll returned %d users, want 1000", len(all))
	}
	if _, ok := handler.find("data/gorm: find all limit applied"); ok {
		t.Fatal("FindAll with exactly 1000 rows emitted a false-positive warning")
	}
}

func TestRepositoryFindAllWithoutLimitReturnsAllRowsAndSkipsWarning(t *testing.T) {
	ctx := context.Background()
	handler := &captureHandler{}
	setDefaultLoggerForTest(t, slog.New(handler))

	db := openIntegrationDB(t)
	truncateUsers(t, db)
	repo := datagorm.NewRepository[integrationUser, int](db, datagorm.WithoutLimit())
	seedManyUsers(t, repo, 1005)

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1005 {
		t.Fatalf("FindAll returned %d users, want all 1005", len(all))
	}
	if _, ok := handler.find("data/gorm: find all limit applied"); ok {
		t.Fatal("FindAll with WithoutLimit logged default limit warning")
	}
}

func TestRepositoryWithTransactionPreservesFindAllLimitOptions(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	truncateUsers(t, db)
	repo := datagorm.NewRepository[integrationUser, int](db, datagorm.WithoutLimit())
	seedManyUsers(t, repo, 1005)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	txRepo := repo.WithTransaction(datagorm.NewTransaction(tx))
	all, err := txRepo.FindAll(ctx)
	if err != nil {
		t.Fatalf("transactional FindAll returned error: %v", err)
	}
	if len(all) != 1005 {
		t.Fatalf("transactional FindAll returned %d users, want all 1005", len(all))
	}
}

func TestRepositoryFindWhereTranslatesPortableFilters(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := datagorm.NewRepository[integrationUser, int](db)
	nick := "countess"
	seedUsers(t, repo,
		integrationUser{Email: "ada@example.test", Name: "Ada_% Lovelace", Age: 36, Nickname: &nick},
		integrationUser{Email: "grace@example.test", Name: "Grace Hopper", Age: 85},
		integrationUser{Email: "alan@example.test", Name: "Alan Turing", Age: 41},
	)

	tests := []struct {
		name      string
		filter    data.Filter
		wantNames []string
	}{
		{
			name: "equal",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: "ada@example.test"},
			),
			wantNames: []string{"Ada_% Lovelace"},
		},
		{
			name: "not equal and greater than",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Email", Operator: data.OperatorNotEqual, Value: "ada@example.test"},
				data.Condition{Field: "Age", Operator: data.OperatorGreaterThan, Value: 50},
			),
			wantNames: []string{"Grace Hopper"},
		},
		{
			name: "greater than or equal",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Age", Operator: data.OperatorGreaterThanOrEqual, Value: 41},
			),
			wantNames: []string{"Alan Turing", "Grace Hopper"},
		},
		{
			name: "less than",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Age", Operator: data.OperatorLessThan, Value: 40},
			),
			wantNames: []string{"Ada_% Lovelace"},
		},
		{
			name: "less than or equal",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Age", Operator: data.OperatorLessThanOrEqual, Value: 36},
			),
			wantNames: []string{"Ada_% Lovelace"},
		},
		{
			name: "contains escapes wildcards",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Name", Operator: data.OperatorContains, Value: "_%"},
			),
			wantNames: []string{"Ada_% Lovelace"},
		},
		{
			name: "in",
			filter: mustFilter(t, data.LogicalAnd,
				data.Condition{Field: "Email", Operator: data.OperatorIn, Value: []string{"ada@example.test", "alan@example.test"}},
			),
			wantNames: []string{"Ada_% Lovelace", "Alan Turing"},
		},
		{
			name: "is null",
			filter: data.Filter{
				Logic: data.LogicalAnd,
				Conditions: []data.Condition{
					{Field: "Nickname", Operator: data.OperatorIsNull},
				},
			},
			wantNames: []string{"Alan Turing", "Grace Hopper"},
		},
		{
			name: "is not null",
			filter: data.Filter{
				Logic: data.LogicalAnd,
				Conditions: []data.Condition{
					{Field: "Nickname", Operator: data.OperatorIsNotNull},
				},
			},
			wantNames: []string{"Ada_% Lovelace"},
		},
		{
			name: "or",
			filter: mustFilter(t, data.LogicalOr,
				data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: "ada@example.test"},
				data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: "grace@example.test"},
			),
			wantNames: []string{"Ada_% Lovelace", "Grace Hopper"},
		},
		{
			name:      "empty filter",
			filter:    data.Filter{},
			wantNames: []string{"Ada_% Lovelace", "Alan Turing", "Grace Hopper"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindWhere(ctx, tt.filter)
			if err != nil {
				t.Fatalf("FindWhere returned error: %v", err)
			}
			assertNames(t, got, tt.wantNames)
		})
	}

	unsafeFilter := data.Filter{
		Logic: data.LogicalAnd,
		Conditions: []data.Condition{
			{Field: "Name; DROP TABLE integration_users", Operator: data.OperatorEqual, Value: "Ada"},
		},
	}
	if _, err := repo.FindWhere(ctx, unsafeFilter); !errors.Is(err, data.ErrInvalidCondition) {
		t.Fatalf("unsafe field error = %v, want ErrInvalidCondition", err)
	} else if !errors.Is(err, data.ErrInvalidFilter) {
		t.Fatalf("unsafe field error = %v, want ErrInvalidFilter compatibility", err)
	}

	if _, err := datagorm.ColumnFor[integrationUser](db, "Name; DROP TABLE integration_users"); !errors.Is(err, data.ErrInvalidCondition) {
		t.Fatalf("unsafe ColumnFor field error = %v, want ErrInvalidCondition", err)
	} else if !errors.Is(err, data.ErrInvalidFilter) {
		t.Fatalf("unsafe ColumnFor field error = %v, want ErrInvalidFilter compatibility", err)
	}
}

func TestRepositoryFindWhereContainsEscapesPercent(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := datagorm.NewRepository[integrationUser, int](db)
	seedUsers(t, repo,
		integrationUser{Email: "promo@example.test", Name: "Promo 50% off", Age: 10},
		integrationUser{Email: "near-promo@example.test", Name: "Promo 500 off", Age: 11},
	)

	got, err := repo.FindWhere(ctx, mustFilter(t, data.LogicalAnd,
		data.Condition{Field: "Name", Operator: data.OperatorContains, Value: "50% off"},
	))
	if err != nil {
		t.Fatalf("FindWhere returned error: %v", err)
	}
	assertNames(t, got, []string{"Promo 50% off"})
}

func TestRepositoryPagination(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := datagorm.NewRepository[integrationUser, int](db)
	users := make([]integrationUser, 0, 25)
	for i := 1; i <= 25; i++ {
		users = append(users, integrationUser{
			Email: fmt.Sprintf("user-%02d@example.test", i),
			Name:  fmt.Sprintf("User %02d", i),
			Age:   i,
		})
	}
	seedUsers(t, repo, users...)

	page, err := repo.Paginate(ctx, 2, 20)
	if err != nil {
		t.Fatalf("Paginate returned error: %v", err)
	}
	if page.Total != 25 || page.Page != 2 || page.PageSize != 20 {
		t.Fatalf("Paginate metadata = total:%d page:%d size:%d, want 25/2/20", page.Total, page.Page, page.PageSize)
	}
	if len(page.Items) != 5 {
		t.Fatalf("Paginate returned %d items, want 5", len(page.Items))
	}

	for _, tc := range []struct {
		page int
		size int
	}{
		{page: 0, size: 20},
		{page: 1, size: 0},
		{page: -1, size: 20},
	} {
		if _, err := repo.Paginate(ctx, tc.page, tc.size); !errors.Is(err, data.ErrInvalidPagination) {
			t.Fatalf("Paginate(%d, %d) error = %v, want ErrInvalidPagination", tc.page, tc.size, err)
		}
	}

	capped, err := repo.Paginate(ctx, 1, 1001)
	if err != nil {
		t.Fatalf("Paginate capped size returned error: %v", err)
	}
	if capped.PageSize != 1000 {
		t.Fatalf("Paginate capped PageSize = %d, want 1000", capped.PageSize)
	}
}

func TestRepositoryWithTransaction(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := datagorm.NewRepository[integrationUser, int](db)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	txRepo := repo.WithTransaction(datagorm.NewTransaction(tx))
	if txRepo == repo {
		t.Fatal("WithTransaction returned the original repository")
	}
	if err := txRepo.Save(ctx, &integrationUser{Email: "tx@example.test", Name: "Tx User"}); err != nil {
		t.Fatalf("Save in transaction returned error: %v", err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}
	tx = nil

	afterRollback, err := repo.FindWhere(ctx, mustFilter(t, data.LogicalAnd,
		data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: "tx@example.test"},
	))
	if err != nil {
		t.Fatalf("original repository should remain usable after rollback, got error: %v", err)
	}
	if len(afterRollback) != 0 {
		t.Fatalf("original repository saw %d rolled back rows, want 0", len(afterRollback))
	}

	invalidTxRepo := repo.WithTransaction(datagorm.NewTransaction(nil))
	if _, err := invalidTxRepo.FindAll(ctx); err == nil {
		t.Fatal("FindAll on nil transaction repository returned nil error")
	}
}

func TestTransactionManagerCommitRollbackAndContextPropagation(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	manager := datagorm.NewTransactionManager(db)
	repo := datagorm.NewRepository[integrationUser, int](db)

	if err := manager.WithinTransaction(ctx, func(txCtx context.Context, tx data.Transaction[*gormlib.DB]) error {
		if tx == nil || tx.Unwrap() == nil {
			t.Fatal("transaction callback received nil transaction")
		}
		if _, ok := data.TransactionFromContext[*gormlib.DB](txCtx); !ok {
			t.Fatal("transaction context does not contain active transaction")
		}
		return repo.Save(txCtx, &integrationUser{Email: "commit@example.test", Name: "Commit User"})
	}); err != nil {
		t.Fatalf("WithinTransaction commit returned error: %v", err)
	}
	assertEmailCount(t, repo, "commit@example.test", 1)

	sentinel := errors.New("rollback sentinel")
	err := manager.WithinTransaction(ctx, func(txCtx context.Context, tx data.Transaction[*gormlib.DB]) error {
		if err := repo.Save(txCtx, &integrationUser{Email: "rollback@example.test", Name: "Rollback User"}); err != nil {
			return err
		}

		nestedErr := manager.WithinTransaction(txCtx, func(nestedCtx context.Context, nestedTx data.Transaction[*gormlib.DB]) error {
			if nestedTx.Unwrap() != tx.Unwrap() {
				t.Fatal("nested transaction did not reuse existing transaction")
			}
			return repo.Save(nestedCtx, &integrationUser{Email: "nested-rollback@example.test", Name: "Nested Rollback User"})
		})
		if nestedErr != nil {
			return nestedErr
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTransaction rollback error = %v, want sentinel", err)
	}
	assertEmailCount(t, repo, "rollback@example.test", 0)
	assertEmailCount(t, repo, "nested-rollback@example.test", 0)
}

func TestDatabaseUsesTransactionFromContext(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	manager := datagorm.NewTransactionManager(db)
	repo := datagorm.NewRepository[integrationUser, int](db)

	err := manager.WithinTransaction(ctx, func(txCtx context.Context, _ data.Transaction[*gormlib.DB]) error {
		txDB, err := datagorm.Database(txCtx, db, "test database helper")
		if err != nil {
			return err
		}
		if err := txDB.Create(&integrationUser{Email: "database-helper@example.test", Name: "Database Helper"}).Error; err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("WithinTransaction returned nil error")
	}
	assertEmailCount(t, repo, "database-helper@example.test", 0)
}

func truncateUsers(t *testing.T, db *gormlib.DB) {
	t.Helper()
	if err := db.Session(&gormlib.Session{AllowGlobalUpdate: true}).Delete(&integrationUser{}).Error; err != nil {
		t.Fatalf("truncate users: %v", err)
	}
}

func openIntegrationDB(t *testing.T) *gormlib.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gormlib.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gormlib.Config{
		TranslateError: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&integrationUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedUsers(t *testing.T, repo *datagorm.Repository[integrationUser, int], users ...integrationUser) {
	t.Helper()

	ctx := context.Background()
	for i := range users {
		if err := repo.Save(ctx, &users[i]); err != nil {
			t.Fatalf("seed user %q: %v", users[i].Email, err)
		}
	}
}

func seedManyUsers(t *testing.T, repo *datagorm.Repository[integrationUser, int], count int) {
	t.Helper()

	users := make([]integrationUser, 0, count)
	for i := 1; i <= count; i++ {
		users = append(users, integrationUser{
			Email: fmt.Sprintf("bulk-%04d@example.test", i),
			Name:  fmt.Sprintf("Bulk %04d", i),
			Age:   i,
		})
	}
	seedUsers(t, repo, users...)
}

func mustFilter(t *testing.T, logic data.LogicalOperator, conditions ...data.Condition) data.Filter {
	t.Helper()

	filter, err := data.NewFilter(logic, conditions...)
	if err != nil {
		t.Fatalf("build filter: %v", err)
	}
	return filter
}

func assertNames(t *testing.T, users []integrationUser, want []string) {
	t.Helper()

	got := make([]string, 0, len(users))
	for _, user := range users {
		got = append(got, user.Name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func assertEmailCount(t *testing.T, repo *datagorm.Repository[integrationUser, int], email string, want int) {
	t.Helper()

	got, err := repo.FindWhere(context.Background(), mustFilter(t, data.LogicalAnd,
		data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: email},
	))
	if err != nil {
		t.Fatalf("FindWhere %q returned error: %v", email, err)
	}
	if len(got) != want {
		t.Fatalf("FindWhere %q returned %d rows, want %d", email, len(got), want)
	}
}

type captureHandler struct {
	state   *captureState
	attrs   []slog.Attr
	groups  []string
	stateMu sync.Mutex
}

type captureState struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	state := h.sharedState()
	state.mu.Lock()
	defer state.mu.Unlock()

	cloned := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	if len(h.attrs) > 0 {
		cloned.AddAttrs(h.attrs...)
	}
	recordAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		recordAttrs = append(recordAttrs, attr)
		return true
	})
	if len(recordAttrs) > 0 {
		cloned.AddAttrs(groupCaptureAttrs(h.groups, recordAttrs)...)
	}
	state.records = append(state.records, cloned)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	clone.attrs = append(clone.attrs, groupCaptureAttrs(clone.groups, attrs)...)
	return clone
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *captureHandler) find(message string) (slog.Record, bool) {
	state := h.sharedState()
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, record := range state.records {
		if record.Message == message {
			return record, true
		}
	}
	return slog.Record{}, false
}

func (h *captureHandler) clone() *captureHandler {
	attrs := make([]slog.Attr, len(h.attrs))
	copy(attrs, h.attrs)
	groups := make([]string, len(h.groups))
	copy(groups, h.groups)
	return &captureHandler{
		state:  h.sharedState(),
		attrs:  attrs,
		groups: groups,
	}
}

func (h *captureHandler) sharedState() *captureState {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.state == nil {
		h.state = &captureState{}
	}
	return h.state
}

func groupCaptureAttrs(groups []string, attrs []slog.Attr) []slog.Attr {
	if len(groups) == 0 {
		return attrs
	}
	current := attrs
	for i := len(groups) - 1; i >= 0; i-- {
		current = []slog.Attr{slog.Group(groups[i], captureAttrsToAny(current)...)}
	}
	return current
}

func captureAttrsToAny(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return args
}

func attrValue(record slog.Record, key string) any {
	var value any
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != key {
			return true
		}
		value = attr.Value.Any()
		return false
	})
	return value
}
