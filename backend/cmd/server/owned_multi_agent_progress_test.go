package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestOwnedMultiAgentProgressUsesExistingContinuityWithoutCompletingGoal(t *testing.T) {
	port := setupMeshLocalDataPort(t)
	db := port.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), port, continuityTestModel{})
	request := business.Request{SpaceID: "core", DeviceID: "device-a", TargetDeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "progress-create"}
	document, _, err := engine.Continuity(t.Context(), request, business.ContinuityMutation{Action: "create", Title: "持续检查"})
	if err != nil {
		t.Fatal(err)
	}
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "core", CharacterID: "one", ConversationID: "conversation"})
	parent.UpdatedAt = time.Now().UTC()
	parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{ParentTurn: &interaction.ParentTurnRecoveryRef{ThreadID: document.Thread.ID}}
	ref := interaction.MultiAgentRecoveryRef{OwnedScope: &document.Scope, CoordinationID: "coord-progress", ResourceOwnerID: document.OwnerID, Status: "succeeded", AssignmentRefs: []interaction.AssignmentRecoveryRef{{Status: "succeeded"}}}
	runtime := newOwnedContinuityRuntime(engine, nil, nil, "core", "device-a", "core")
	for i := 0; i < 2; i++ {
		if err := runtime.ApplyMultiAgentProgress(t.Context(), parent, ref); err != nil {
			t.Fatal(err)
		}
	}
	request.RequestID = "progress-read"
	current, _, err := engine.Continuity(t.Context(), request, business.ContinuityMutation{Action: "read", ID: document.Thread.ID})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range current.Events {
		if event.EventType != "multi_agent.progress" {
			continue
		}
		count++
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["parentInteractionId"] != parent.ID || payload["verification"] != "requires_parent_review" {
			t.Fatalf("progress lost parent evidence: %+v", payload)
		}
	}
	if count != 1 || current.Thread.Status != continuity.ThreadStatusActive {
		t.Fatalf("projection duplicated progress or completed goal: events=%d status=%s", count, current.Thread.Status)
	}
	foreign := *ref.OwnedScope
	foreign.RoleID = "other"
	ref.OwnedScope = &foreign
	if err := runtime.ApplyMultiAgentProgress(t.Context(), parent, ref); err == nil {
		t.Fatal("foreign role projected progress")
	}
}
