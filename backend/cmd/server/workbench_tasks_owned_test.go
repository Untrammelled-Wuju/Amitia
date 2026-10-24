package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/workbenchtasks"
)

func registerWorkbenchFixture(f *threeCoreFixture, tracker interaction.InteractionTracker) {
	rt := f.services.DeviceMesh
	authenticate := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: f.core, DeviceCredentials: rt.CredentialSvc, DeviceRegistry: rt.DeviceReg, Coordination: rt.Coordination})
	workbenchtasks.NewHandler(tracker).WithConversationAccess(workbenchConversationAccess(f.services, f.core)).RegisterRoutes(f.router.Group("/api", authenticate))
}

func seedWorkbenchTask(t *testing.T, tracker interaction.InteractionTracker, core, owner, conv, request, task string) {
	t.Helper()
	record := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: core, ConversationID: conv, CharacterID: "one", RequestID: request})
	record.RecoveryDescriptor = &interaction.RecoveryDescriptor{MultiAgent: &interaction.MultiAgentRecoveryRef{ExecutionCoreID: core, ResourceOwnerID: owner, CoordinationID: task, Status: "running", ParentGoalSpaceID: core, ParentGoalCharacterID: "one", ParentGoalConversationID: conv, AssignmentRefs: []interaction.AssignmentRecoveryRef{{AssignmentID: "worker", WorkerID: "worker", CharacterID: "one", Role: "coding_worker", Status: "running", Objective: "check source"}}}}
	if err := tracker.Create(t.Context(), record); err != nil {
		t.Fatal(err)
	}
}

func TestWorkbenchOwnedTasksCloudDeviceAndContinuationHTTP(t *testing.T) {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		t.Setenv(key, "")
	}
	t.Setenv("NO_PROXY", "*")
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "core-a", schemas), newThreeCoreFixture(t, "core-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	tracker := interaction.NewInMemoryTracker()
	registerWorkbenchFixture(b, tracker)
	var turn struct {
		Data struct {
			ConversationID string `json:"conversationId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "task-turn-device", "characterId": "one", "conversationId": "same-id", "message": "hello"}, 200), &turn); err != nil {
		t.Fatal(err)
	}
	seedWorkbenchTask(t, tracker, b.core, a.device.DeviceID.String(), turn.Data.ConversationID, "task-turn-device", "device-task")
	seedWorkbenchTask(t, tracker, b.core, "other-owner", turn.Data.ConversationID, "task-turn-device", "other-owner-task")
	query := url.Values{"conversationId": {"same-id"}, "conversationOwnerId": {a.device.DeviceID.String()}, "characterId": {"one"}, "expectedCoreId": {b.core}}
	var list workbenchtasks.TaskList
	if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 200), &list); err != nil || len(list.Tasks) != 1 || list.Tasks[0].CoordinationID != "device-task" || list.QueryContext == nil || list.QueryContext.ExecutionScope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatalf("device task projection: %+v %v", list, err)
	}
	query.Set("coordinationId", "device-task")
	var history workbenchtasks.TaskCheckpointHistory
	if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/workbench/tasks/checkpoints?"+query.Encode(), nil, 200), &history); err != nil || len(history.Records) != 1 {
		t.Fatalf("checkpoint: %+v %v", history, err)
	}
	query.Set("coordinationId", "other-owner-task")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks/checkpoints?"+query.Encode(), nil, 404)
	query.Set("conversationOwnerId", "foreign-owner")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 403)
	query.Set("conversationOwnerId", a.device.DeviceID.String())
	query.Set("expectedCoreId", "old-core")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 409)
	query.Set("expectedCoreId", b.core)
	query.Set("characterId", "disabled")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 403)
	query.Set("characterId", "one")
	query.Set("conversationId", "missing")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 404)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	if err := json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "task-turn-core", "characterId": "one", "conversationId": "same-id", "conversationOrigin": map[string]string{"ownerId": a.device.DeviceID.String(), "id": "same-id"}, "message": "continue"}, 200), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.Data.ConversationID == "same-id" || turn.Data.ConversationID == "" {
		t.Fatalf("continuation not created: %+v", turn)
	}
	seedWorkbenchTask(t, tracker, b.core, b.core, turn.Data.ConversationID, "task-turn-core", "core-task")
	query.Set("conversationId", "same-id")
	if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 200), &list); err != nil || len(list.Tasks) != 1 || list.Tasks[0].CoordinationID != "core-task" || list.ConversationID != turn.Data.ConversationID || list.QueryContext.ExecutionScope.ResourceOwnerID != b.core {
		t.Fatalf("continuation task: %+v %v", list, err)
	}
	query.Set("coordinationId", "core-task")
	b.request(t, a, http.MethodGet, "/api/workbench/tasks/checkpoints?"+query.Encode(), nil, 200)
}
