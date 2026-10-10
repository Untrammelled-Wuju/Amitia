package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
)

func installGenericParentCheckpoint(t *testing.T, db *gorm.DB, parentID string) {
	t.Helper()
	var record interaction.InteractionRecordModel
	if err := db.Where("id = ?", parentID).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	desc := &interaction.RecoveryDescriptor{
		SchemaVersion: interaction.RecoveryDescriptorSchemaVersion,
		Requirement:   interaction.RecoveryRequired,
		Interaction: interaction.RecoveryInteractionRef{
			InteractionID: record.ID, RequestID: record.RequestID,
			Status: interaction.InteractionStatus(record.Status), StatusVersion: record.StatusVersion,
		},
		Scope: interaction.RecoveryScopeRef{
			SpaceID: record.SpaceID, CharacterID: record.CharacterID,
			ConversationID: record.ConversationID, Channel: record.Channel,
		},
		ParentTurn: &interaction.ParentTurnRecoveryRef{
			ThreadID: "thread-1", MessageStyle: "compact",
			WorkspaceID: "workspace-1", PermissionMode: "request_approval",
			WorkspaceName: "Engineering", WorkspaceKind: "local",
			WorkspaceRootURI: "file:///D:/project", WorkspaceDeviceID: "device-1",
			ModelConfigID: 42, ReasoningEffort: "medium",
			ConversationSnapshot: &interaction.ParentTurnConversationSnapshot{
				ModelConfigID: 42, ReasoningEffort: "medium", ReasoningEnabled: 1,
				PermissionMode: "request_approval", WorkspaceID: "workspace-1",
			},
		},
	}
	desc.ComputeFingerprint()
	value, err := json.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&interaction.InteractionRecordModel{}).Where("id = ?", parentID).
		Update("recovery_descriptor_json", string(value)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestGenericParentTurnRecoveryWithoutMultiAgent(t *testing.T) {
	db, parentID, turnID := setupParentRecoveryCandidate(t)
	installGenericParentCheckpoint(t, db, parentID)
	got, err := findRecoverableParentTurns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].parentID != parentID ||
		got[0].request.TurnID != turnID || got[0].request.WorkspaceID != "workspace-1" ||
		got[0].request.ExecutionID != "exec-1" || got[0].request.ModelConfigID != 42 ||
		got[0].request.WorkspaceDeviceID != "device-1" ||
		got[0].request.WorkspaceRootURI != "file:///D:/project" ||
		got[0].request.ThreadID != "thread-1" ||
		got[0].request.MessageStyle != "compact" {
		t.Fatalf("generic parent not recoverable with stable identity: %+v", got)
	}
}

func TestGenericParentTurnRecoveryFailsClosedOnDrift(t *testing.T) {
	for _, scenario := range []string{"workspace", "device", "permission", "model", "effort", "reasoning", "tampered", "unresolved_tool"} {
		t.Run(scenario, func(t *testing.T) {
			db, parentID, turnID := setupParentRecoveryCandidate(t)
			installGenericParentCheckpoint(t, db, parentID)
			switch scenario {
			case "workspace":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").
					Update("workspace_id", "new-workspace").Error; err != nil {
					t.Fatal(err)
				}
			case "device":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").
					Update("workspace_device_id", "different-device").Error; err != nil {
					t.Fatal(err)
				}
			case "permission":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").
					Update("permission_mode", "full_access").Error; err != nil {
					t.Fatal(err)
				}
			case "effort":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").Update("reasoning_effort", "high").Error; err != nil {
					t.Fatal(err)
				}
			case "reasoning":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").Update("reasoning_enabled", 0).Error; err != nil {
					t.Fatal(err)
				}
			case "model":
				if err := db.Model(&chat.Conversation{}).Where("id = ?", "conversation-1").
					Update("model_config_id", 43).Error; err != nil {
					t.Fatal(err)
				}
			case "tampered":
				var record interaction.InteractionRecordModel
				if err := db.Where("id = ?", parentID).First(&record).Error; err != nil {
					t.Fatal(err)
				}
				var desc interaction.RecoveryDescriptor
				if err := json.Unmarshal([]byte(record.RecoveryDescriptorJSON), &desc); err != nil {
					t.Fatal(err)
				}
				desc.ParentTurn.WorkspaceID = "attacker-workspace"
				value, err := json.Marshal(desc)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Model(&interaction.InteractionRecordModel{}).Where("id = ?", parentID).
					Update("recovery_descriptor_json", string(value)).Error; err != nil {
					t.Fatal(err)
				}
			case "unresolved_tool":
				if err := db.Create(&chat.AssistantTurnItem{
					ID: "in-flight", TurnID: turnID, ConversationID: "conversation-1",
					ItemType: "tool_call", CallID: "effect-1", ToolName: "execute_host_command",
					Status: "running", ArgumentsJSON: `{"command":"touch marker"}`,
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			got, err := findRecoverableParentTurns(context.Background(), db)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Fatalf("unsafe generic parent resumed in %s: %+v", scenario, got)
			}
		})
	}
}

func TestGenericParentTurnRecoveryRestartsOriginalInteraction(t *testing.T) {
	db, parentID, turnID := setupParentRecoveryCandidate(t)
	installGenericParentCheckpoint(t, db, parentID)
	tracker := interaction.NewSQLiteInteractionTracker(db)
	proc := &testRecoveredParentProcessor{}
	orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), proc, tracker, nil)
	orch.SetReady(true)
	entry := interaction.NewUnifiedEntry(orch, interaction.NewScopeResolver(nil), temporal.SystemClock{})
	entry.SetExecutionService(coreexec.NewExecutionService())
	runner := newParentTurnRecoveryRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runner.Scan(ctx, ctx, &AppServices{DB: db, UnifiedEntry: entry}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	record, found, err := tracker.Get(ctx, parentID)
	if err != nil || !found || record.Status != interaction.InteractionStatusCompleted {
		t.Fatalf("generic parent did not commit original interaction %q: %+v %v", parentID, record, err)
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("generic original turn %s replayed %d times", turnID, proc.calls.Load())
	}
	if err := runner.Scan(ctx, ctx, &AppServices{DB: db, UnifiedEntry: entry}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("terminal generic parent executed repeatedly: %d", proc.calls.Load())
	}
}

func TestGenericParentTurnRecoveryPreservesExplicitModelOverride(t *testing.T) {
	db, parentID, _ := setupParentRecoveryCandidate(t)
	installGenericParentCheckpoint(t, db, parentID)
	var record interaction.InteractionRecordModel
	if err := db.Where("id = ?", parentID).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	var desc interaction.RecoveryDescriptor
	if err := json.Unmarshal([]byte(record.RecoveryDescriptorJSON), &desc); err != nil {
		t.Fatal(err)
	}
	desc.ParentTurn.ModelConfigID = 19
	desc.ParentTurn.ReasoningEffort = "high"
	desc.ComputeFingerprint()
	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&interaction.InteractionRecordModel{}).Where("id = ?", parentID).
		Update("recovery_descriptor_json", string(data)).Error; err != nil {
		t.Fatal(err)
	}
	got, err := findRecoverableParentTurns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].request.ModelConfigID != 19 ||
		got[0].request.ReasoningEffort != "high" {
		t.Fatalf("model override was lost or incorrectly rejected by conversation baseline: %+v", got)
	}
}
