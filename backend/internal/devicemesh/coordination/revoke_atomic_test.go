package coordination_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRevokeSerializesWithCommitAndRejectsLateCommit(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "core", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	entered, release := make(chan struct{}), make(chan struct{})
	committed := make(chan error, 1)
	go func() {
		committed <- coordination.CommitCurrent(ctx, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	revoked := make(chan error, 1)
	go func() {
		revoked <- service.RevokeDevice(t.Context(), "space", "b", func(tx *sql.Tx) error {
			_, err := tx.Exec(`UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='b'`)
			return err
		})
	}()
	select {
	case err := <-revoked:
		close(release)
		t.Fatalf("revocation crossed active commit: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-committed; err != nil {
		t.Fatal(err)
	}
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("target revocation did not cancel caller")
	}
	if err := service.Validate(t.Context(), scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revoked scope accepted: %v", err)
	}
	if err := coordination.CommitCurrent(ctx, func() error { t.Fatal("late commit executed"); return nil }); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("late commit: %v", err)
	}
	var state string
	if err := db.QueryRow(`SELECT trust_state FROM kernel_devices WHERE device_id='b'`).Scan(&state); err != nil || state != "revoked" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestRevokeFailureRollsBackPolicyAndTrust(t *testing.T) {
	db, service := setup(t)
	policy, err := service.ChangeMode(t.Context(), "space", "a", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "", "core", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	failure := errors.New("pairing transaction failed")
	err = service.RevokeDevice(t.Context(), "space", "a", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='a'`); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	latest, err := service.Get(t.Context(), "space", "a")
	if err != nil || latest != policy {
		t.Fatalf("policy changed on failure: %+v %v", latest, err)
	}
	if err := service.Validate(ctx, scope); err != nil {
		t.Fatalf("valid scope canceled on rollback: %v", err)
	}
	var state string
	if err := db.QueryRow(`SELECT trust_state FROM kernel_devices WHERE device_id='a'`).Scan(&state); err != nil || state != "trusted" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}
