package dbtransaction

import (
	"context"
	"database/sql"
	"testing"
)

func TestTransactionBindingMatchesDatabaseAndPreservesOuterBinding(t *testing.T) {
	first, second := &sql.DB{}, &sql.DB{}
	firstTx, secondTx := &sql.Tx{}, &sql.Tx{}
	ctx := With(context.Background(), first, firstTx)
	if _, ok := From(ctx, second); ok {
		t.Fatal("transaction escaped database boundary")
	}
	ctx = With(ctx, second, secondTx)
	if found, ok := From(ctx, first); !ok || found != firstTx {
		t.Fatal("outer database transaction lost")
	}
	if found, ok := From(ctx, second); !ok || found != secondTx {
		t.Fatal("inner database transaction lost")
	}
	if found, ok := Current(ctx); !ok || found != secondTx {
		t.Fatal("current binding differs")
	}
}
