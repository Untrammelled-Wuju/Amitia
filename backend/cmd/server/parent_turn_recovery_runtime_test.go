package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
)

func setupParentRecoveryCandidate(t *testing.T) (*gorm.DB, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "turn-recovery.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tracker := interaction.NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&chat.AssistantTurn{}, &chat.AssistantTurnItem{}, &chat.Message{}, &chat.Conversation{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	conversation := &chat.Conversation{
		ID: "conversation-1", SpaceID: "space-1", Channel: "web", WorkspaceID: "workspace-1",
		ModelConfigID: 42, ReasoningEffort: "medium", ReasoningEnabled: 1,
		PermissionMode: "request_approval",
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{
		SpaceID: "space-1", CharacterID: "character-1", ConversationID: "conversation-1",
		RequestID: "request-1", Channel: "web", Source: "web",
	})
	parent.Status = interaction.InteractionStatusContextReady
	desc := &interaction.RecoveryDescriptor{
		SchemaVersion: interaction.RecoveryDescriptorSchemaVersion,
		MultiAgent: &interaction.MultiAgentRecoveryRef{
			CoordinationID: "coord-1", ParentGoalID: "goal-1", ParentGoalRevision: 1,
			ParentGoalSpaceID: "space-1", ParentGoalCharacterID: "character-1",
			ParentGoalConversationID: "conversation-1",
			Status:                   "succeeded", Strategy: interaction.CoordinationParallel,
			CompletionPlan: interaction.CoordinationRequireAll, WorkspaceID: "workspace-1",
			PermissionMode: "request_approval",
		},
	}
	desc.ComputeFingerprint()
	parent.RecoveryDescriptor = desc
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	user := chat.Message{ID: "user-1", ConversationID: "conversation-1", CharacterID: "character-1",
		Role: "user", Content: "Finish the coding task", MsgType: "text", RequestID: "request-1"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	turn := chat.AssistantTurn{ID: "turn-1", ConversationID: "conversation-1", CharacterID: "character-1",
		UserMessageID: user.ID, RequestID: "request-1", ExecutionID: "exec-1",
		Status: "waiting_tool", Sequence: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	return db, parent.ID, turn.ID
}

func TestParentTurnRecoverySelectsOriginalTurnIdentity(t *testing.T) {
	db, parentID, turnID := setupParentRecoveryCandidate(t)
	got, err := findRecoverableParentTurns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one recoverable parent, got %d", len(got))
	}
	req := got[0].request
	if got[0].parentID != parentID || req.TurnID != turnID || req.ExecutionID != "exec-1" ||
		!req.RecoverExistingTurn || req.ReservedInteractionID != parentID ||
		req.WorkspaceID != "workspace-1" || req.PermissionMode != "request_approval" ||
		req.Message != "Finish the coding task" || req.ModelConfigID != 42 ||
		req.ReasoningEffort != "medium" || req.ReasoningEnabled == nil || !*req.ReasoningEnabled {
		t.Fatalf("lost original turn identity or authorization: %+v", req)
	}
}

func TestParentTurnRecoveryRejectsUnresolvedToolCalls(t *testing.T) {
	db, _, turnID := setupParentRecoveryCandidate(t)
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Create(&chat.AssistantTurnItem{ID: "tool-1", TurnID: turnID,
		ConversationID: "conversation-1", ItemType: "tool_call", Status: "running",
		CallID: "call-1", ToolName: "execute_host_command", ArgumentsJSON: `{"command":"go test ./..."}`,
		CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := findRecoverableParentTurns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatal("unacknowledged tool execution was replayable")
	}
	result := chat.AssistantTurnItem{ID: "result-1", TurnID: turnID,
		ConversationID: "conversation-1", ItemType: "tool_result", Status: "completed",
		CallID: "call-1", ResultJSON: `{"exitCode":0}`, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&result).Error; err != nil {
		t.Fatal(err)
	}
	got, err = findRecoverableParentTurns(context.Background(), db)
	if err != nil || len(got) != 1 {
		t.Fatalf("fully acknowledged tool call should allow continuation: %v %+v", err, got)
	}
}

func TestParentTurnRecoveryRefusesAlreadyCommittedOrSupersededWork(t *testing.T) {
	for _, scenario := range []string{"already_committed", "newer_interaction", "nonterminal_children", "tampered_descriptor", "workspace_mismatch", "deleted_conversation"} {
		t.Run(scenario, func(t *testing.T) {
			db, parentID, _ := setupParentRecoveryCandidate(t)
			switch scenario {
			case "deleted_conversation":
				if err := db.Delete(&chat.Conversation{}, "id = ?", "conversation-1").Error; err != nil {
					t.Fatal(err)
				}
			case "workspace_mismatch":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").
					Update("workspace_id", "different-workspace").Error; err != nil {
					t.Fatal(err)
				}
			case "already_committed":
				if err := db.Create(&chat.Message{ID: "assistant-1", ConversationID: "conversation-1",
					CharacterID: "character-1", Role: "assistant", Content: "done",
					RequestID: "request-1"}).Error; err != nil {
					t.Fatal(err)
				}
			case "newer_interaction":
				newer := interaction.NewInteractionRecord(interaction.InteractionScope{
					SpaceID: "space-1", CharacterID: "character-1",
					ConversationID: "conversation-1", RequestID: "request-2", Channel: "web",
				})
				newer.CreatedAt = time.Now().UTC().Add(time.Minute)
				tracker := interaction.NewSQLiteInteractionTracker(db)
				if err := tracker.Create(context.Background(), newer); err != nil {
					t.Fatal(err)
				}
			case "nonterminal_children", "tampered_descriptor":
				var parent interaction.InteractionRecordModel
				if err := db.Where("id = ?", parentID).First(&parent).Error; err != nil {
					t.Fatal(err)
				}
				var ref interaction.RecoveryDescriptor
				if err := json.Unmarshal([]byte(parent.RecoveryDescriptorJSON), &ref); err != nil {
					t.Fatal(err)
				}
				if scenario == "nonterminal_children" {
					ref.MultiAgent.Status = "running"
					ref.ComputeFingerprint()
				} else {
					ref.MultiAgent.WorkspaceID = "attacker-workspace"
				}
				b, err := json.Marshal(ref)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Model(&interaction.InteractionRecordModel{}).
					Where("id = ?", parentID).Update("recovery_descriptor_json", string(b)).Error; err != nil {
					t.Fatal(err)
				}
			}
			got, err := findRecoverableParentTurns(context.Background(), db)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Fatalf("unsafe recovery candidate accepted in %s: %+v", scenario, got)
			}
		})
	}
}

func TestParentTurnJournalRequiresMatchingAcknowledgements(t *testing.T) {
	for _, items := range [][]chat.AssistantTurnItem{
		{{ItemType: "tool_call", CallID: "call"}},
		{{ItemType: "tool_result", CallID: "orphan", ResultJSON: `{}`}},
		{{ItemType: "tool_call", CallID: "call"}, {ItemType: "tool_call", CallID: "call"}},
		{{ItemType: "tool_call", CallID: "call"}, {ItemType: "tool_result", CallID: "call", ResultJSON: "not json"}},
	} {
		if parentTurnJournalSafeToResume(items) {
			t.Fatalf("incomplete journal considered safe: %+v", items)
		}
	}
	if !parentTurnJournalSafeToResume([]chat.AssistantTurnItem{
		{ItemType: "tool_call", CallID: "call"},
		{ItemType: "tool_result", CallID: "call", ResultJSON: `{"exitCode":0}`},
	}) {
		t.Fatal("fully acknowledged result rejected")
	}
}

type testRecoveredParentProcessor struct {
	calls atomic.Int32
	seen  chan string
}

func (p *testRecoveredParentProcessor) ProcessMessageCtx(_ context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	p.calls.Add(1)
	if p.seen != nil {
		select {
		case p.seen <- req.TurnID:
		default:
		}
	}
	if req.ReservedInteractionID == "" || !req.RecoverExistingTurn {
		return nil, fmt.Errorf("parent recovery lost reserved interaction marker")
	}
	if req.ExecContext != nil && req.ExecContext.ExecutionID != req.ExecutionID {
		return nil, fmt.Errorf("parent recovery split original execution identity")
	}
	return &interaction.ProcessResponse{
		ConversationID: req.ConversationID, CharacterID: req.CharacterID,
		RequestID: req.RequestID, Reply: "parent resumed with completed worker evidence",
		MessageIDs: []string{"recovered-answer"},
	}, nil
}

func TestParentTurnRecoveryAutomaticallyResumesPersistedParent(t *testing.T) {
	db, parentID, turnID := setupParentRecoveryCandidate(t)
	tracker := interaction.NewSQLiteInteractionTracker(db)
	proc := &testRecoveredParentProcessor{seen: make(chan string, 1)}
	orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), proc, tracker, nil)
	orch.SetReady(true)
	entry := interaction.NewUnifiedEntry(orch, interaction.NewScopeResolver(nil), temporal.SystemClock{})
	entry.SetExecutionService(coreexec.NewExecutionService())
	svc := &AppServices{DB: db, UnifiedEntry: entry}
	runner := newParentTurnRecoveryRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runner.Scan(ctx, ctx, svc); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-proc.seen:
		if got != turnID {
			t.Fatalf("resumed a new turn instead of the original: %s", got)
		}
	case <-ctx.Done():
		t.Fatal("automatic parent recovery never reached original processor")
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	stored, found, err := tracker.Get(ctx, parentID)
	if err != nil || !found || stored.Status != interaction.InteractionStatusCompleted {
		t.Fatalf("resumed parent did not commit to original interaction: %+v %v", stored, err)
	}
	if err := runner.Scan(ctx, ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("automatic scan replayed a completed parent turn: %d", proc.calls.Load())
	}
}

func TestParentTurnRecoveryScansPastIneligibleFirstBatch(t *testing.T) {
	db, parentID, _ := setupParentRecoveryCandidate(t)
	for i := 0; i < 96; i++ {
		decoy := interaction.InteractionRecordModel{
			ID:                     fmt.Sprintf("00-decoy-%03d", i),
			Status:                 string(interaction.InteractionStatusContextReady),
			RecoveryDescriptorJSON: "{}",
		}
		if err := db.Create(&decoy).Error; err != nil {
			t.Fatal(err)
		}
	}
	got, err := findRecoverableParentTurns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].parentID != parentID {
		t.Fatalf("scan lost valid parent behind ineligible records: %+v", got)
	}
}

func TestParentTurnRecoveryRetriesPreExecutionReadinessFailure(t *testing.T) {
	db, parentID, _ := setupParentRecoveryCandidate(t)
	tracker := interaction.NewSQLiteInteractionTracker(db)
	proc := &testRecoveredParentProcessor{}
	orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), proc, tracker, nil)
	entry := interaction.NewUnifiedEntry(orch, interaction.NewScopeResolver(nil), temporal.SystemClock{})
	svc := &AppServices{DB: db, UnifiedEntry: entry}
	runner := newParentTurnRecoveryRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runner.Scan(ctx, ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	runner.mu.Lock()
	retryAt, ok := runner.attempted[parentID]
	runner.mu.Unlock()
	if !ok || !retryAt.After(time.Now()) || proc.calls.Load() != 0 {
		t.Fatalf("pre-execution transient failure did not schedule safe retry: at=%s ok=%v calls=%d",
			retryAt, ok, proc.calls.Load())
	}
	orch.SetReady(true)
	runner.mu.Lock()
	runner.attempted[parentID] = time.Now().Add(-time.Second)
	runner.mu.Unlock()
	if err := runner.Scan(ctx, ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("safe readiness retry did not resume the original task: %d", proc.calls.Load())
	}
}
