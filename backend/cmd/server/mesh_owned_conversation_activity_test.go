package main

import (
	"encoding/json"
	"testing"
)

func TestOwnedReminderActivityUsesSavedMessageAndReadPosition(t *testing.T) {
	port := setupMeshLocalDataPort(t)
	db := port.services.KernelContainer.DeviceRegistry.Database()
	_, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources
		(owner_id, kind, resource_id, role_id, revision, deleted, body, updated_at)
		VALUES (?, 'message', 'reminder-message', 'role', 1, 0, ?, ?)`,
		port.ownerID, `{"conversationId":"reminder-conversation","role":"assistant","source":"reminder","content":"该喝水了","createdAt":"2026-11-20T08:00:00.000000Z"}`, "2026-11-20T08:00:00.000000Z")
	if err != nil {
		t.Fatal(err)
	}
	input := []json.RawMessage{json.RawMessage(`{"id":"reminder-conversation","lastReadTurnSequence":0}`)}
	output, err := port.enrichOwnedConversationActivity(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(output[0], &document); err != nil {
		t.Fatal(err)
	}
	sequence, _ := document["lastTerminalTurnSequence"].(float64)
	if sequence <= 0 || document["hasUnread"] != true || document["unreadCompletedCount"] != float64(1) || document["lastMessage"] != "该喝水了" || document["source"] != "reminder" {
		t.Fatalf("unexpected unread activity: %+v", document)
	}
	document["lastReadTurnSequence"] = sequence
	read, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	output, err = port.enrichOwnedConversationActivity(t.Context(), []json.RawMessage{read})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output[0], &document); err != nil {
		t.Fatal(err)
	}
	if document["hasUnread"] != false || document["unreadCompletedCount"] != float64(0) {
		t.Fatalf("read reminder remained unread: %+v", document)
	}
	_, err = db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources
		(owner_id, kind, resource_id, role_id, revision, deleted, body, updated_at)
		VALUES (?, 'checkpoint', 'turn/later', 'role', 1, 0, ?, ?)`,
		port.ownerID, `{"conversationId":"reminder-conversation","status":"completed"}`, "2026-11-20T08:01:00.000000Z")
	if err != nil {
		t.Fatal(err)
	}
	output, err = port.enrichOwnedConversationActivity(t.Context(), []json.RawMessage{read})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output[0], &document); err != nil {
		t.Fatal(err)
	}
	if document["hasUnread"] != true || document["unreadCompletedCount"] != float64(1) {
		t.Fatalf("later response was not unread: %+v", document)
	}
}
