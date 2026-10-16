package coordination

import (
	"context"
	"database/sql"

	"github.com/u-ai/backend/internal/dbtransaction"
)

type queryExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func currentQueryExecutor(ctx context.Context, db *sql.DB) queryExecutor {
	if tx, ok := dbtransaction.From(ctx, db); ok {
		return tx
	}
	return db
}

type heldAuthorityKey struct{}

func CommitCurrentContext(ctx context.Context, commit func(context.Context) error) error {
	if _, _, readOnly := TaskReadAuthority(ctx); readOnly {
		return ErrWrongOwner
	}
	service, _ := ctx.Value(authorityServiceKey{}).(*Service)
	if service == nil {
		if err := ValidateCurrent(ctx); err != nil {
			return err
		}
		return commit(ctx)
	}
	return CommitCurrent(ctx, func() error {
		return commit(context.WithValue(ctx, heldAuthorityKey{}, service))
	})
}
