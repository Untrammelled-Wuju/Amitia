package dbtransaction

import (
	"context"
	"database/sql"
)

type transactionKey struct{}

type binding struct {
	db       *sql.DB
	tx       *sql.Tx
	previous *binding
}

func With(ctx context.Context, db *sql.DB, tx *sql.Tx) context.Context {
	previous, _ := ctx.Value(transactionKey{}).(*binding)
	return context.WithValue(ctx, transactionKey{}, &binding{db: db, tx: tx, previous: previous})
}

func From(ctx context.Context, db *sql.DB) (*sql.Tx, bool) {
	current, _ := ctx.Value(transactionKey{}).(*binding)
	for current != nil {
		if current.db == db && current.tx != nil {
			return current.tx, true
		}
		current = current.previous
	}
	return nil, false
}

func Current(ctx context.Context) (*sql.Tx, bool) {
	current, _ := ctx.Value(transactionKey{}).(*binding)
	if current == nil || current.tx == nil {
		return nil, false
	}
	return current.tx, true
}
