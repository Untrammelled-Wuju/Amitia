package coordination_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPendingCommitRetainsOriginalCallerAuthorityAcrossRestartAndDropsRevokedInvitation(t *testing.T) {
	db, service := setup(t)
	_, sender, senderFinish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "original-invitation")
	if err != nil {
		t.Fatal(err)
	}
	defer senderFinish()
	_, recipient, recipientFinish, err := service.Begin(t.Context(), "space", "b", "b", "space", "role", "pending-invitation")
	if err != nil {
		t.Fatal(err)
	}
	defer recipientFinish()
	sender.RoleRevision, recipient.RoleRevision = 1, 1
	commit := coordination.Commit{Scope: recipient, AdditionalAuthorities: []coordination.ExecutionScope{sender}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: "realtime-invitation/original", RoleID: "role", Body: json.RawMessage(`{"status":"pending"}`)}}}
	if err := service.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	restarted := coordination.NewService(db)
	pending, err := restarted.Pending(t.Context(), "b")
	if err != nil || len(pending) != 1 || len(pending[0].Commit.AdditionalAuthorities) != 1 || pending[0].Commit.AdditionalAuthorities[0] != sender {
		t.Fatal("outbox lost frozen original caller", pending, err)
	}
	if _, err := service.ChangeMode(t.Context(), "space", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Validate(t.Context(), recipient); err != nil {
		t.Fatal("recipient policy unexpectedly changed", err)
	}
	if err := restarted.ValidateCommitAuthorities(t.Context(), pending[0].Commit); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revoked caller pending invitation stayed valid: %v", err)
	}
	if err := restarted.DiscardPending(t.Context(), pending[0]); err != nil {
		t.Fatal("cannot discard expired original caller invitation", err)
	}
	remaining, err := restarted.Pending(t.Context(), "b")
	if err != nil || len(remaining) != 0 {
		t.Fatal("expired invitation retained for background delivery", remaining, err)
	}
}

func TestCommitAdditionalAuthorityCannotChangeTargetRoleOrOwner(t *testing.T) {
	_, service := setup(t)
	_, recipient, finish, err := service.Begin(t.Context(), "space", "b", "b", "space", "role", "pending")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	recipient.RoleRevision = 1
	other := recipient
	other.ResourceOwnerID = "a"
	commit := coordination.Commit{Scope: recipient, AdditionalAuthorities: []coordination.ExecutionScope{other}}
	if err := service.ValidateCommitAuthorities(t.Context(), commit); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign extra authority accepted: %v", err)
	}
}
