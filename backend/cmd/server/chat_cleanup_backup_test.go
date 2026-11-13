package main

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/system/dataportability"
)

func TestVerifyChatCleanupBackupRejectsEmptyChatComponents(t *testing.T) {
	plan := ownedChatCleanupPlan{Records: []ownedChatManagementRecord{
		{Kind: "conversation", ID: "conv-1"},
		{Kind: "message", ID: "req-1/user"},
		{Kind: "message", ID: "req-1/assistant"},
	}}
	backup := &dataportability.BackupResult{Path: "/tmp/backup.amitia-backup", Manifest: &dataportability.BackupManifest{Components: []dataportability.BackupComponentManifest{
		{ID: chat.ComponentIDChatConversations, ItemCount: 0},
		{ID: chat.ComponentIDChatMessages, ItemCount: 0},
	}}}
	err := verifyChatCleanupBackup(backup, plan)
	if err == nil {
		t.Fatal("expected cleanup to be rejected when chat backup is empty")
	}
	if !strings.Contains(err.Error(), "备份包含 0 个会话/0 条消息") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyChatCleanupBackupAcceptsCompleteChatComponents(t *testing.T) {
	plan := ownedChatCleanupPlan{Records: []ownedChatManagementRecord{
		{Kind: "conversation", ID: "conv-1"},
		{Kind: "message", ID: "req-1/user"},
	}}
	backup := &dataportability.BackupResult{Manifest: &dataportability.BackupManifest{Components: []dataportability.BackupComponentManifest{
		{ID: chat.ComponentIDChatConversations, ItemCount: 4},
		{ID: chat.ComponentIDChatMessages, ItemCount: 9},
	}}}
	if err := verifyChatCleanupBackup(backup, plan); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyChatCleanupBackupRejectsMissingManifest(t *testing.T) {
	if err := verifyChatCleanupBackup(&dataportability.BackupResult{}, ownedChatCleanupPlan{}); err == nil {
		t.Fatal("expected missing manifest to be rejected")
	}
}
