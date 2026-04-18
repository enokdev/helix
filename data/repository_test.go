package data

import (
"context"
"errors"
"testing"
)

type testUser struct {
ID   int
Name string
}

type testRepository struct {
transaction Transaction[string]
}

func (r *testRepository) FindAll(_ context.Context) ([]testUser, error) {
return []testUser{{ID: 1, Name: "Ada"}}, nil
}

func (r *testRepository) FindByID(_ context.Context, id int) (*testUser, error) {
if id != 1 {
return nil, ErrRecordNotFound
}
return &testUser{ID: id, Name: "Ada"}, nil
}

func (r *testRepository) FindWhere(_ context.Context, _ Filter) ([]testUser, error) {
return []testUser{{ID: 1, Name: "Ada"}}, nil
}

func (r *testRepository) Save(_ context.Context, _ *testUser) error {
return nil
}

func (r *testRepository) Delete(_ context.Context, id int) error {
if id != 1 {
return ErrRecordNotFound
}
return nil
}

func (r *testRepository) Paginate(_ context.Context, page, size int) (Page[testUser], error) {
return Page[testUser]{
Items:    []testUser{{ID: 1, Name: "Ada"}},
Total:    1,
Page:     page,
PageSize: size,
}, nil
}

func (r *testRepository) WithTransaction(tx Transaction[string]) Repository[testUser, int, string] {
return &testRepository{transaction: tx}
}

type testTransaction struct {
value string
}

func (t testTransaction) Unwrap() string {
return t.value
}

func TestRepositoryContract(t *testing.T) {
var _ Repository[testUser, int, string] = (*testRepository)(nil)

repo := &testRepository{}
txRepo := repo.WithTransaction(testTransaction{value: "tx"})

if txRepo == repo {
t.Fatal("WithTransaction returned the original repository")
}
if repo.transaction != nil {
t.Fatal("WithTransaction mutated the original repository's transaction field")
}
}

func TestPageCarriesPaginationMetadata(t *testing.T) {
page := Page[testUser]{
Items:    []testUser{{ID: 1, Name: "Ada"}},
Total:    42,
Page:     2,
PageSize: 20,
}

if len(page.Items) != 1 {
t.Fatalf("expected one item, got %d", len(page.Items))
}
if page.Total != 42 {
t.Fatalf("expected total 42, got %d", page.Total)
}
if page.Page != 2 {
t.Fatalf("expected page 2, got %d", page.Page)
}
if page.PageSize != 20 {
t.Fatalf("expected page size 20, got %d", page.PageSize)
}
}

func TestFilterValidation(t *testing.T) {
tests := []struct {
name    string
filter  Filter
wantErr error
}{
{
name:   "zero filter is valid",
filter: Filter{},
},
{
name: "default logic with conditions is valid",
filter: Filter{
Conditions: []Condition{
{Field: "email", Operator: OperatorEqual, Value: "ada@example.test"},
},
},
},
{
name: "equality condition is valid",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "email", Operator: OperatorEqual, Value: "ada@example.test"},
},
},
},
{
name: "contains condition is valid",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "name", Operator: OperatorContains, Value: "Ada"},
},
},
},
{
name: "in condition is valid",
filter: Filter{
Logic: LogicalOr,
Conditions: []Condition{
{Field: "id", Operator: OperatorIn, Value: []int{1, 2}},
},
},
},
{
name: "is null condition does not require value",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "deleted_at", Operator: OperatorIsNull},
},
},
},
{
name: "missing field fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Operator: OperatorEqual, Value: "Ada"},
},
},
wantErr: ErrInvalidFilter,
},
{
name: "unknown operator fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "name", Operator: Operator("raw"), Value: "Ada"},
},
},
wantErr: ErrInvalidFilter,
},
{
name: "missing comparison value fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "age", Operator: OperatorGreaterThan},
},
},
wantErr: ErrInvalidFilter,
},
{
name: "unknown logical operator fails",
filter: Filter{
Logic: LogicalOperator("xor"),
Conditions: []Condition{
{Field: "name", Operator: OperatorEqual, Value: "Ada"},
},
},
wantErr: ErrInvalidFilter,
},
{
name:    "non-default logic with zero conditions fails",
filter:  Filter{Logic: LogicalAnd},
wantErr: ErrInvalidFilter,
},
{
name: "OperatorIn with scalar value fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "id", Operator: OperatorIn, Value: 42},
},
},
wantErr: ErrInvalidFilter,
},
{
name: "OperatorIn with empty slice fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "id", Operator: OperatorIn, Value: []int{}},
},
},
wantErr: ErrInvalidFilter,
},
{
name: "typed nil value fails",
filter: Filter{
Logic: LogicalAnd,
Conditions: []Condition{
{Field: "email", Operator: OperatorEqual, Value: (*string)(nil)},
},
},
wantErr: ErrInvalidFilter,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
err := tt.filter.Validate()
if !errors.Is(err, tt.wantErr) {
t.Fatalf("expected error %v, got %v", tt.wantErr, err)
}
})
}
}

func TestNewFilterValidatesConditions(t *testing.T) {
filter, err := NewFilter(LogicalAnd, Condition{
Field:    "age",
Operator: OperatorGreaterThanOrEqual,
Value:    18,
})
if err != nil {
t.Fatalf("expected valid filter, got %v", err)
}
if len(filter.Conditions) != 1 {
t.Fatalf("expected one condition, got %d", len(filter.Conditions))
}

_, err = NewFilter(LogicalAnd, Condition{Field: "age", Operator: OperatorGreaterThan})
if !errors.Is(err, ErrInvalidFilter) {
t.Fatalf("expected ErrInvalidFilter, got %v", err)
}
}
