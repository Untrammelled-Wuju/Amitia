package proof

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func BindTx(ctx context.Context, tx *sql.Tx, p Proof, core string, body ClaimBody, now time.Time) error {
	if err := Verify(p, core, body, now); err != nil {
		return err
	}
	var key string
	var revoked bool
	err := tx.QueryRowContext(ctx, `SELECT public_key,revoked FROM kernel_device_identity_keys WHERE device_id=?`, body.DeviceID).Scan(&key, &revoked)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && (key != p.PublicKey || revoked) {
		return ErrIdentityCopy
	}
	var other string
	err = tx.QueryRowContext(ctx, `SELECT device_id FROM kernel_device_identity_keys WHERE public_key=?`, p.PublicKey).Scan(&other)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && other != body.DeviceID {
		return ErrIdentityCopy
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES(?,?,?) ON CONFLICT DO NOTHING`, body.DeviceID, p.PublicKey, now.Unix()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_identity_nonces WHERE expires_at<?`, now.Unix()); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_identity_nonces(public_key,nonce,expires_at) VALUES(?,?,?) ON CONFLICT DO NOTHING`, p.PublicKey, p.Nonce, now.Add(2*time.Minute).Unix())
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrProof
	}
	return nil
}
