package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/workbenchtasks"
)

type ownedDelegatingIntegrationModel struct {
	runtime        *meshOwnedToolRuntime
	calls          atomic.Int32
	parentID       string
	coordinationID string
	requestContext context.Context
}

func (m *ownedDelegatingIntegrationModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	m.calls.Add(1)
	current, finish, err := m.runtime.BeginOwnedTurn(ctx, input)
	if err != nil {
		return business.Generation{}, err
	}
	input.RegisterCompletion(finish)
	m.requestContext = current
	tools, err := m.runtime.Tools(current, input)
	if err != nil {
		return business.Generation{}, err
	}
	found := false
	for _, available := range tools {
		if available.Function.Name == multiAgentDelegateTool {
			found = true
		}
	}
	if !found {
		return business.Generation{}, fmt.Errorf("native delegation was not exposed")
	}
	result, err := m.runtime.Execute(current, input, "delegate-call", multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect authorized source module","Validate authorized source module"]}`))
	if err != nil {
		return business.Generation{}, err
	}
	if result.Error != nil {
		return business.Generation{}, fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
	}
	var task business.TaskEvent
	if err := json.Unmarshal(result.Output, &task); err != nil {
		return business.Generation{}, err
	}
	m.coordinationID = task.CoordinationID
	parent, found, err := m.runtime.services.WorkbenchTaskTracker.GetByRequestID(current, input.Scope.SpaceID, ownedParentKey(input))
	if err != nil || !found {
		return business.Generation{}, fmt.Errorf("missing parent: %v", err)
	}
	m.parentID = parent.ID
	return business.Generation{Text: "后台任务已创建，仍需验收", Tokens: 1}, nil
}

func (*ownedDelegatingIntegrationModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}

func TestOwnedMultiAgentHTTPDelegationAndWorkbenchProjection(t *testing.T) {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		t.Setenv(key, "")
	}
	t.Setenv("NO_PROXY", "*")
	schemas := newThreeCoreSchemas(t)
	source, provider := newThreeCoreFixture(t, "owned-integration-source", schemas), newThreeCoreFixture(t, "owned-integration-core", schemas)
	pairThreeCoreFixtures(t, source, provider)
	tracker := interaction.NewInMemoryTracker()
	workers := &harnessTestWorker{tracker: tracker}
	goals := decision.NewGoalRegistry()
	coordinator := interaction.NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, interaction.DefaultMultiAgentPolicy())
	provider.services.WorkbenchTaskTracker, provider.services.MultiAgentCoordinator = tracker, coordinator
	registry := capability.NewToolRegistry()
	provider.services.KernelContainer.ToolRegistry = registry
	provider.services.KernelContainer.ToolFacade = kernel.NewToolFacade(registry, nil)
	runtime := &meshOwnedToolRuntime{services: provider.services, native: &chatMultiAgentRuntime{tracker: tracker, goals: goals, coordinator: coordinator}}
	model := &ownedDelegatingIntegrationModel{runtime: runtime}
	rt := provider.services.DeviceMesh
	provider.services.OwnedBusiness = business.NewEngine(rt.Coordination, rt, model)
	coordinator.SetOwnedExecution(provider.services.OwnedBusiness.WithExecutionScope)
	registerWorkbenchFixture(provider, tracker)
	for _, coordinated := range []bool{false, true} {
		t.Run(fmt.Sprintf("coordinated-%t", coordinated), func(t *testing.T) {
			if coordinated {
				policy, err := rt.Coordination.Get(t.Context(), provider.core, source.device.DeviceID.String())
				if err != nil {
					t.Fatal(err)
				}
				provider.request(t, source, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
			}
			queryScope := decodeThreeCoreMemoryResponse[projectListing](t, provider.request(t, source, http.MethodGet, "/api/device-mesh/v1/business/projects?characterId=one", nil, 200))
			workspace := fmt.Sprintf("owned-integration-%t", coordinated)
			created := decodeThreeCoreMemoryResponse[business.ProjectResponse](t, provider.request(t, source, http.MethodPost, "/api/device-mesh/v1/business/projects", map[string]any{"requestId": "project-" + workspace, "title": workspace, "characterId": "one", "expectedExecutionScope": queryScope.Scope, "workspaces": []business.ProjectWorkspace{{ID: workspace, DeviceID: queryScope.Scope.TargetDeviceID, Name: workspace, Kind: "local", RootURI: "amitia://workspace/@" + workspace + "/"}}, "primaryWorkspaceId": workspace}, 200))
			payload := map[string]any{"requestId": "turn-" + workspace, "conversationId": "conversation-" + workspace, "characterId": "one", "message": "委派两个源码任务", "projectId": created.Project.ID}
			before := len(workers.started)
			var response struct {
				Data business.Response `json:"data"`
			}
			if err := json.Unmarshal(provider.request(t, source, http.MethodPost, "/api/device-mesh/v1/business/messages", payload, 200), &response); err != nil {
				t.Fatal(err)
			}
			if !response.Data.Saved || len(workers.started) != before+2 {
				t.Fatalf("chat did not launch workers: %+v starts=%d", response.Data, len(workers.started))
			}
			owner := source.device.DeviceID.String()
			if coordinated {
				owner = provider.core
			}
			for _, request := range workers.started[before:] {
				if request.OwnedScope == nil || request.OwnedScope.ResourceOwnerID != owner || request.WorkspaceID != workspace {
					t.Fatalf("worker lost owned authority: %+v", request)
				}
			}
			parent, found, err := tracker.Get(t.Context(), model.parentID)
			if err != nil || !found || parent.IsTerminal() {
				t.Fatalf("HTTP completion closed running parent: %+v %v", parent, err)
			}
			if parent.RecoveryDescriptor.OwnedTurn.Scope.ResourceOwnerID != owner {
				t.Fatal("parent owner mismatch")
			}
			select {
			case <-model.requestContext.Done():
			case <-time.After(time.Second):
				t.Fatal("completed HTTP request did not release its execution context")
			}
			if err := coordinator.Reconcile(t.Context(), model.coordinationID); err != nil {
				t.Fatalf("background reconciliation lost request authority: %v", err)
			}
			calls := model.calls.Load()
			provider.request(t, source, http.MethodPost, "/api/device-mesh/v1/business/messages", payload, 200)
			if model.calls.Load() != calls || len(workers.started) != before+2 {
				t.Fatal("retry dispatched duplicate workers")
			}
			query := url.Values{"conversationId": {response.Data.ConversationID}, "conversationOwnerId": {owner}, "characterId": {"one"}, "expectedCoreId": {provider.core}}
			var list workbenchtasks.TaskList
			if err := json.Unmarshal(provider.request(t, source, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 200), &list); err != nil || len(list.Tasks) != 1 || list.Tasks[0].CoordinationID != model.coordinationID {
				t.Fatalf("real task projection failed: %+v %v", list, err)
			}
			query.Set("coordinationId", model.coordinationID)
			var history workbenchtasks.TaskCheckpointHistory
			if err := json.Unmarshal(provider.request(t, source, http.MethodGet, "/api/workbench/tasks/checkpoints?"+query.Encode(), nil, 200), &history); err != nil || len(history.Records) == 0 {
				t.Fatalf("real checkpoint missing: %+v %v", history, err)
			}
			query.Set("conversationOwnerId", "foreign-owner")
			provider.request(t, source, http.MethodGet, "/api/workbench/tasks?"+query.Encode(), nil, 403)
			if err := coordinator.Cancel(t.Context(), model.coordinationID); err != nil {
				t.Fatal(err)
			}
			if err := coordinator.Reconcile(t.Context(), model.coordinationID); err != nil {
				t.Fatal(err)
			}
			parent, _, err = tracker.Get(t.Context(), model.parentID)
			if err != nil || parent.Status != interaction.InteractionStatusCancelled {
				t.Fatalf("explicit cancellation did not close parent: %+v %v", parent, err)
			}
		})
	}
}
