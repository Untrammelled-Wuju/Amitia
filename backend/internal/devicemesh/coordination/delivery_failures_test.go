package coordination_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPermanentDeliveryFailurePurgesPayloadAndSurvivesRestart(t *testing.T) {
	db, service := setup(t)
	commit := ownedCommit("rejected")
	commit.Mutations[0].Body = json.RawMessage(`{"conversationId":"conversation","private":"must-not-remain-at-Core"}`)
	if err := service.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	pending, err := service.PendingRequest(t.Context(), "a", "rejected")
	if err != nil || pending == nil {
		t.Fatal(err)
	}
	if err := service.RejectPending(t.Context(), *pending, errors.New("source offline")); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("temporary offline failure was treated as permanent: %v", err)
	}
	if err := service.RejectPending(t.Context(), *pending, coordination.ErrResourceVersion); err != nil {
		t.Fatal(err)
	}
	restarted := coordination.NewService(db)
	if payload, err := restarted.PendingRequest(t.Context(), "a", "rejected"); err != nil || payload != nil {
		t.Fatal("private outbox payload survived permanent failure")
	}
	failures, err := restarted.DeliveryFailures(t.Context(), commit.Scope, "conversation")
	if err != nil || len(failures) != 1 || failures[0].ErrorCode != coordination.ProtocolErrorCode(coordination.ErrResourceVersion) {
		t.Fatalf("failure was lost after restart: %+v %v", failures, err)
	}
	encoded, _ := json.Marshal(failures)
	if strings.Contains(string(encoded), "must-not-remain-at-Core") {
		t.Fatal("private payload was copied into failure metadata")
	}
	var columns int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('kernel_device_owned_delivery_failures') WHERE name IN ('payload','body','scope','result')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("failure metadata table has a private payload column")
	}
	if err := restarted.Enqueue(t.Context(), commit); !errors.Is(err, coordination.ErrDeliveryRejected) || !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("permanent request was requeued: %v", err)
	}
	commit.Mutations[0].Body = json.RawMessage(`{"private":"changed"}`)
	if err := restarted.Enqueue(t.Context(), commit); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("rejected request identity was reused: %v", err)
	}
}

func TestDeliveryFailuresAreScopedToCallerOwnerRoleAndConversation(t *testing.T) {
	_, service := setup(t)
	commit := ownedCommit("namespaced-failure")
	commit.Mutations[0].Body = json.RawMessage(`{"conversationId":"one"}`)
	if err := service.Enqueue(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	pending, err := service.PendingRequest(t.Context(), "a", commit.Scope.RequestID)
	if err != nil || pending == nil {
		t.Fatal(err)
	}
	changed := *pending
	changed.Hash = "wrong"
	if err := service.RejectPending(t.Context(), changed, coordination.ErrResourceVersion); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("wrong payload fingerprint was accepted: %v", err)
	}
	if err := service.RejectPending(t.Context(), *pending, coordination.ErrRequestConflict); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.DeliveryFailures(t.Context(), commit.Scope, "two"); err != nil || len(rows) != 0 {
		t.Fatalf("failure leaked to another conversation: %+v %v", rows, err)
	}
	otherRole := commit.Scope
	otherRole.RoleID = "other-role"
	if rows, err := service.DeliveryFailures(t.Context(), otherRole, "one"); err != nil || len(rows) != 0 {
		t.Fatalf("failure leaked to another role: %+v %v", rows, err)
	}
	otherCaller := commit.Scope
	otherCaller.InitiatorDeviceID = "b"
	if rows, err := service.DeliveryFailures(t.Context(), otherCaller, "one"); err != nil || len(rows) != 0 {
		t.Fatalf("failure leaked to another caller: %+v %v", rows, err)
	}
}
