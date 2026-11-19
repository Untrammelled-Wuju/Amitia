package main

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedChatArchiveKeepsBooleanAndTimestampInSync(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	ctx := t.Context()
	scope := prepareTestScope(p, "archive-scope")
	create := coordination.ChatManagementRequest{Operation: "conversation/create", ConversationID: "conv-archive", ConversationOwnerID: p.ownerID, RequestID: "archive-create", Title: "归档会话"}
	if _, err := p.ManageChat(ctx, scope, create); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	on := true
	request := create
	request.Operation = "conversation/archive"
	request.RequestID = "archive-on"
	request.Archived = &on
	result, err := p.ManageChat(ctx, scope, request)
	if err != nil {
		t.Fatalf("archive conversation: %v", err)
	}
	if result["archived"] != true {
		t.Fatalf("archive flag not written: %+v", result)
	}
	if archivedAt, _ := result["archivedAt"].(string); archivedAt == "" {
		t.Fatalf("archivedAt not written: %+v", result)
	}
	resource, err := p.store.Get(ctx, "conversation", "conv-archive")
	if err != nil || resource == nil {
		t.Fatalf("read conversation: %+v %v", resource, err)
	}
	if coordination.ResourceAvailable(resource.Body, time.Now(), false) {
		t.Fatal("archived conversation still reported available")
	}
	off := false
	request.RequestID = "archive-off"
	request.Archived = &off
	result, err = p.ManageChat(ctx, scope, request)
	if err != nil {
		t.Fatalf("unarchive conversation: %v", err)
	}
	if result["archived"] != false {
		t.Fatalf("archive flag not cleared: %+v", result)
	}
	if archivedAt, _ := result["archivedAt"].(string); archivedAt != "" {
		t.Fatalf("archivedAt not cleared: %+v", result)
	}
	resource, err = p.store.Get(ctx, "conversation", "conv-archive")
	if err != nil || resource == nil {
		t.Fatalf("read unarchived conversation: %+v %v", resource, err)
	}
	if !coordination.ResourceAvailable(resource.Body, time.Now(), false) {
		t.Fatalf("unarchived conversation still unavailable: %s", string(resource.Body))
	}
}
