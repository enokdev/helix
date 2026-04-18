package data

import (
	"context"
	"errors"
	"testing"
)

type testTX struct {
	id string
}

type contextTestTransaction struct {
	tx *testTX
}

func (t contextTestTransaction) Unwrap() *testTX {
	return t.tx
}

func TestContextTransactionRoundTrip(t *testing.T) {
	ctx := context.Background()
	tx := contextTestTransaction{tx: &testTX{id: "tx-1"}}

	txCtx, err := ContextWithTransaction[*testTX](ctx, tx)
	if err != nil {
		t.Fatalf("ContextWithTransaction returned error: %v", err)
	}

	got, ok := TransactionFromContext[*testTX](txCtx)
	if !ok {
		t.Fatal("TransactionFromContext did not find transaction")
	}
	if got.Unwrap().id != "tx-1" {
		t.Fatalf("transaction id = %q, want tx-1", got.Unwrap().id)
	}

	if _, ok := TransactionFromContext[string](txCtx); ok {
		t.Fatal("TransactionFromContext returned transaction for wrong TX type")
	}
}

func TestContextWithTransactionRejectsInvalidInputs(t *testing.T) {
	if _, err := ContextWithTransaction[*testTX](context.TODO(), contextTestTransaction{tx: &testTX{}}); err != nil {
		t.Fatalf("ContextWithTransaction valid inputs returned error: %v", err)
	}
	if _, err := ContextWithTransaction[*testTX](nil, contextTestTransaction{tx: &testTX{}}); err == nil {
		t.Fatal("ContextWithTransaction nil context returned nil error")
	}

	if _, err := ContextWithTransaction[*testTX](context.Background(), nil); err == nil {
		t.Fatal("ContextWithTransaction nil transaction returned nil error")
	}

	var nilConcrete *contextTestTransaction
	if _, err := ContextWithTransaction[*testTX](context.Background(), nilConcrete); err == nil {
		t.Fatal("ContextWithTransaction nil concrete transaction returned nil error")
	}
}

func TestTransactionManagerContract(_ *testing.T) {
	var _ TransactionManager[*testTX] = testTransactionManager{}
}

type testTransactionManager struct{}

func (testTransactionManager) WithinTransaction(context.Context, func(context.Context, Transaction[*testTX]) error) error {
	return errors.New("not implemented")
}
