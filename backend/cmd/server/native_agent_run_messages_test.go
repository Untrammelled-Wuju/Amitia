package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agentrun"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func testAgentRunMessageRuntime(t *testing.T) (*chatMultiAgentRuntime, chat.SkillScope, agentrun.Message) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:native-agent-run-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	service, err := agentrun.NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SyncCoordination(context.Background(), agentrun.Coordination{
		SpaceID: "space-a", ConversationID: "conversation-a", CoordinationID: "coordination-a",
		Status: agentrun.StatusRunning,
		Assignments: []agentrun.Assignment{{
			AssignmentID: "assignment-a", CharacterID: "role-a", WorkerRole: "coding_worker",
			Objective: "verify message delivery", Status: agentrun.StatusRunning,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	message, err := service.Send(context.Background(), agentrun.SendInput{
		SpaceID: "space-a", SenderRunID: agentrun.RootRunID("coordination-a"),
		RecipientID: agentrun.ChildRunID("assignment-a"), Kind: agentrun.MessageInstruction,
		Content: "complete the focused verification",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &chatMultiAgentRuntime{
		tracker:   interaction.NewInMemoryTracker(),
		agentRuns: service,
	}
	scope := chat.SkillScope{
		SpaceID: "space-a", ConversationID: "conversation-a", CharacterID: "role-a",
		RequestID: "ma-assignment-a", Source: "multi_agent",
	}
	return runtime, scope, message
}

func TestNativeAgentRunClaimAcknowledgeAndNoRedelivery(t *testing.T) {
	runtime, scope, message := testAgentRunMessageRuntime(t)
	read := runtime.executeAgentRunMessageTool(
		context.Background(),
		agentRunReadMessagesTool,
		json.RawMessage(`{"lease_seconds":30,"limit":5}`),
		scope,
	)
	if read.Error != nil || read.Status != "SUCCESS" {
		t.Fatalf("claim failed: %+v", read)
	}
	var claim agentrun.Claim
	if err := json.Unmarshal(read.Output, &claim); err != nil {
		t.Fatal(err)
	}
	if claim.LeaseToken == "" || len(claim.Messages) != 1 || claim.Messages[0].ID != message.ID {
		t.Fatalf("unexpected tool claim: %+v", claim)
	}

	ackInput, err := json.Marshal(map[string]any{
		"lease_token": claim.LeaseToken,
		"message_ids": []string{message.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	ack := runtime.executeAgentRunMessageTool(
		context.Background(),
		agentRunAcknowledgeMessagesTool,
		ackInput,
		scope,
	)
	if ack.Error != nil || ack.Status != "SUCCESS" {
		t.Fatalf("acknowledge failed: %+v", ack)
	}
	var acknowledgement struct {
		Count int64 `json:"acknowledgedCount"`
	}
	if err := json.Unmarshal(ack.Output, &acknowledgement); err != nil {
		t.Fatal(err)
	}
	if acknowledgement.Count != 1 {
		t.Fatalf("acknowledged=%d, want 1", acknowledgement.Count)
	}

	second := runtime.executeAgentRunMessageTool(
		context.Background(),
		agentRunReadMessagesTool,
		json.RawMessage(`{}`),
		scope,
	)
	if second.Error != nil || second.Status != "SUCCESS" {
		t.Fatalf("second claim failed: %+v", second)
	}
	var empty agentrun.Claim
	if err := json.Unmarshal(second.Output, &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Messages) != 0 {
		t.Fatalf("acknowledged message was redelivered: %+v", empty.Messages)
	}
}

func TestNativeAgentRunStaleLeaseIsRejectedWithOriginalError(t *testing.T) {
	runtime, scope, message := testAgentRunMessageRuntime(t)
	read := runtime.executeAgentRunMessageTool(
		context.Background(),
		agentRunReadMessagesTool,
		json.RawMessage(`{}`),
		scope,
	)
	if read.Error != nil {
		t.Fatalf("claim failed: %+v", read)
	}
	input, err := json.Marshal(map[string]any{
		"lease_token": "stale-lease", "message_ids": []string{message.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	ack := runtime.executeAgentRunMessageTool(
		context.Background(),
		agentRunAcknowledgeMessagesTool,
		input,
		scope,
	)
	if ack.Error == nil || ack.Error.Code != "AGENT_MESSAGE_ACKNOWLEDGE_FAILED" ||
		ack.Error.Message == "" {
		t.Fatalf("stale lease was not rejected with the original error: %+v", ack)
	}
}
