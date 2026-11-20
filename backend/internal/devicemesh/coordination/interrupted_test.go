package coordination_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func interruptedScope() coordination.ExecutionScope {
	return coordination.ExecutionScope{
		SpaceID:            "space",
		InitiatorDeviceID:  "a",
		TargetDeviceID:     "a",
		ModeRevision:       1,
		PermissionRevision: 1,
		RoleID:             "role-1",
		RoleOwnerID:        "a",
		ResourceOwnerID:    "a",
		RequestID:          "req-1",
		TurnID:             "turn-1",
		ExecutionID:        "exec-1",
	}
}

func seedInterruptedTurn(t *testing.T, db *sql.DB, scope coordination.ExecutionScope) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation := map[string]any{"id": "conv-1", "conversationId": "conv-1", "title": "question", "characterId": "role-1", "lastMessage": "question", "lastMessagePreview": "question", "lastMessageRole": "user", "lastMessageType": "text", "lastMessageSequence": 3, "updatedAt": now}
	conversationBody, err := json.Marshal(conversation)
	if err != nil {
		t.Fatal(err)
	}
	checkpointBody, err := json.Marshal(map[string]any{"hash": "hash-1", "status": "running", "conversationId": "conv-1", "scope": scope, "reason": ""})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','conversation','conv-1','role-1',4,?,?)`, []byte(conversationBody), now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','checkpoint','turn/req-1','role-1',1,?,?)`, []byte(checkpointBody), now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func ownedResourceBody(t *testing.T, db *sql.DB, kind, id string) map[string]any {
	t.Helper()
	var raw []byte
	if err := db.QueryRow(`SELECT body FROM kernel_device_owned_resources WHERE owner_id='a' AND kind=? AND resource_id=? AND deleted=0`, kind, id).Scan(&raw); err != nil {
		t.Fatalf("%s/%s missing: %v", kind, id, err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestInterruptedReplyUpdatesConversationProjectionAndSequence(t *testing.T) {
	db, _ := setup(t)
	scope := interruptedScope()
	seedInterruptedTurn(t, db, scope)
	store := coordination.NewOwnershipStore(db, "a")
	if err := store.SaveInterrupted(context.Background(), coordination.InterruptedReply{Scope: scope, ConversationID: "conv-1", Text: "已经写了一半的回复", Reasoning: "思考片段", Reason: "用户取消"}); err != nil {
		t.Fatal(err)
	}

	conversation := ownedResourceBody(t, db, "conversation", "conv-1")
	if conversation["lastMessageRole"] != "assistant" {
		t.Fatalf("lastMessageRole=%v", conversation["lastMessageRole"])
	}
	if conversation["lastMessage"] != "已经写了一半的回复" || conversation["lastMessagePreview"] != "已经写了一半的回复" {
		t.Fatalf("lastMessage=%v preview=%v", conversation["lastMessage"], conversation["lastMessagePreview"])
	}
	if conversation["lastMessageType"] != "text" {
		t.Fatalf("lastMessageType=%v", conversation["lastMessageType"])
	}
	if sequence, ok := conversation["lastMessageSequence"].(float64); !ok || int64(sequence) != 3 {
		t.Fatalf("lastMessageSequence=%v", conversation["lastMessageSequence"])
	}

	message := ownedResourceBody(t, db, "message", "req-1/assistant")
	if message["status"] != "interrupted" || message["content"] != "已经写了一半的回复" {
		t.Fatalf("message=%v", message)
	}
	if sequence, ok := message["sequence"].(float64); !ok || int64(sequence) != 3 {
		t.Fatalf("assistant sequence=%v", message["sequence"])
	}

	var conversationRevision int64
	if err := db.QueryRow(`SELECT revision FROM kernel_device_owned_resources WHERE owner_id='a' AND kind='conversation' AND resource_id='conv-1'`).Scan(&conversationRevision); err != nil {
		t.Fatal(err)
	}
	if conversationRevision != 5 {
		t.Fatalf("conversation revision=%d", conversationRevision)
	}
}

func TestInterruptedReplyWithoutTextKeepsLatestUserPreviewAndIsIdempotent(t *testing.T) {
	db, _ := setup(t)
	scope := interruptedScope()
	seedInterruptedTurn(t, db, scope)
	store := coordination.NewOwnershipStore(db, "a")
	reply := coordination.InterruptedReply{Scope: scope, ConversationID: "conv-1", Reason: "用户取消"}
	if err := store.SaveInterrupted(context.Background(), reply); err != nil {
		t.Fatal(err)
	}
	conversation := ownedResourceBody(t, db, "conversation", "conv-1")
	if conversation["lastMessageRole"] != "user" || conversation["lastMessage"] != "question" {
		t.Fatalf("empty interrupted reply changed preview: %v", conversation)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id='a' AND kind='message' AND resource_id='req-1/assistant'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("empty interrupted reply wrote %d messages", count)
	}
	if err := store.SaveInterrupted(context.Background(), reply); err != nil {
		t.Fatalf("repeat not idempotent: %v", err)
	}
}

func TestInterruptedReplyMismatchedReasonIsRejected(t *testing.T) {
	db, _ := setup(t)
	scope := interruptedScope()
	seedInterruptedTurn(t, db, scope)
	store := coordination.NewOwnershipStore(db, "a")
	first := coordination.InterruptedReply{Scope: scope, ConversationID: "conv-1", Text: "部分内容", Reason: "用户取消"}
	if err := store.SaveInterrupted(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Reason = "网络中断"
	if err := store.SaveInterrupted(context.Background(), second); err != coordination.ErrRequestConflict {
		t.Fatalf("mismatched reason err=%v", err)
	}
	message := ownedResourceBody(t, db, "message", "req-1/assistant")
	if message["content"] != "部分内容" {
		t.Fatalf("message overwritten: %v", message)
	}
}
