package coordination_test

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestCurrentRemoteAuthorityUsesScopedTargetAndDurableConfirmation(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "tool")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, _, err := coordination.TrackCurrentRemoteAuthority(ctx, "space", "a", "session", 1); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("wrong device accepted: %v", err)
	}
	if _, _, err := coordination.TrackCurrentRemoteAuthority(coordination.WithScope(t.Context(), scope), "space", "b", "session", 1); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unvalidated scope accepted: %v", err)
	}
	id, confirm, err := coordination.TrackCurrentRemoteAuthority(ctx, "space", "b", "session", 1)
	if err != nil || id == "" || confirm == nil {
		t.Fatalf("registration failed: %s %v", id, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_remote_authority WHERE call_id=? AND caller_id='a' AND target_id='b'`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("durable record missing: %d %v", count, err)
	}
	if err := confirm(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_remote_authority WHERE call_id=?`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("confirmed authority retained: %d %v", count, err)
	}
}
