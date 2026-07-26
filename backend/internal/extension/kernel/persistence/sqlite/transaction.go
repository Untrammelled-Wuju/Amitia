package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/glebarez/sqlite"
)

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type Store struct {
	db *sql.DB
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", dsn, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: set pragmas: %w", err)
	}
	return &Store{db: db}, nil
}

func NewStoreFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	return Migrate(ctx, s.db)
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

type txKey struct{}

type TransactionManager struct {
	db *sql.DB
}

func NewTransactionManager(db *sql.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

func (tm *TransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return fn(ctx)
	}

	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	err = fn(txCtx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("sqlite: rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit transaction: %w", err)
	}

	return nil
}

func getExecutor(ctx context.Context, db *sql.DB) Executor {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return db
}
