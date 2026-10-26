package kernel

import (
	"context"
	"database/sql"
	"errors"

	"github.com/u-ai/backend/internal/extension/kernel/secret"
)

func purgeRetiredSecretRefs(ctx context.Context, db *sql.DB, broker *secret.Broker) error {
	if db == nil || broker == nil {
		return nil
	}
	rows, err := db.QueryContext(ctx, "SELECT secret_ref FROM retired_secret_cleanup")
	if err != nil {
		return nil
	}
	refs := make([]string, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, raw)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, raw := range refs {
		ref, err := secret.ParseRef(raw)
		if err != nil {
			return err
		}
		if err := broker.Delete(ctx, ref); err != nil && !errors.Is(err, secret.ErrSecretNotFound) {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM retired_secret_cleanup"); err != nil {
		return err
	}
	return nil
}
