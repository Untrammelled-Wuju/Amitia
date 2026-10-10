package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/interaction"
)

const multiAgentDelegateTool = "harness_multi_agent_delegate"
const multiAgentStatusTool = "harness_multi_agent_status"
const multiAgentWaitTool = "harness_multi_agent_wait"

type chatMultiAgentRuntime struct {
	coordinator *interaction.MultiAgentCoordinator
	tracker     interaction.InteractionTracker
	goals       *decision.GoalRegistry
	mu          sync.Mutex
	dispatches  map[string]*multiAgentDispatchLock
}

type multiAgentDispatchLock struct {
	mu      sync.Mutex
	waiters int
}

func (a *chatToolRuntimeAdapter) ConfigureMultiAgent(coordinator *interaction.MultiAgentCoordinator, tracker interaction.InteractionTracker, goals *decision.GoalRegistry) {
	a.multiAgent = &chatMultiAgentRuntime{coordinator: coordinator, tracker: tracker, goals: goals}
}

func isChatMultiAgentTool(name string) bool {
	return name == multiAgentDelegateTool || name == multiAgentStatusTool || name == multiAgentWaitTool
}

func (a *chatToolRuntimeAdapter) multiAgentAvailable(ctx context.Context, scope chat.SkillScope) bool {
	if a == nil || a.multiAgent == nil || a.multiAgent.coordinator == nil ||
		a.multiAgent.tracker == nil || a.multiAgent.goals == nil || scope.ExecContext == nil {
		return false
	}
	if strings.TrimSpace(scope.ExecContext.WorkspaceID) == "" {
		return false
	}
	if strings.TrimSpace(scope.SpaceID) == "" || strings.TrimSpace(scope.CharacterID) == "" ||
		strings.TrimSpace(scope.ConversationID) == "" || strings.TrimSpace(scope.RequestID) == "" ||
		scope.IsInternal || scope.Source == "multi_agent" {
		return false
	}
	parent, found, err := a.multiAgent.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
	if err != nil || !found || parent == nil || parent.IsTerminal() {
		return false
	}
	actual := parent.Scope.Normalize()
	return actual.SpaceID == scope.SpaceID && actual.CharacterID == scope.CharacterID &&
		actual.ConversationID == scope.ConversationID && actual.RequestID == scope.RequestID &&
		actual.Source != "multi_agent" && actual.Source != "runtime" && actual.Source != "proactive" &&
		(scope.Channel == "" || actual.Channel == scope.Channel)
}

func multiAgentToolParameters(schema string) tool.Parameters {
	var value tool.Parameters
	_ = json.Unmarshal([]byte(schema), &value)
	return value
}

func (a *chatToolRuntimeAdapter) appendMultiAgentTools(ctx context.Context, existing []tool.Tool, scope chat.SkillScope) []tool.Tool {
	if !a.multiAgentAvailable(ctx, scope) {
		return existing
	}
	for _, def := range existing {
		if isChatMultiAgentTool(def.Function.Name) {
			return existing
		}
	}
	return append(existing,
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentDelegateTool,
			Description: "Delegate two to four independent coding subtasks within the already authorized workspace to the existing Multi-Agent coordinator. Returns task IDs, not proof of completion. Do not dispatch a second batch until the first batch is finished.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"objectives":{"type":"array","minItems":2,"maxItems":4,"items":{"type":"string","minLength":8,"maxLength":2000}},"strategy":{"type":"string","enum":["parallel","sequential"]}},"required":["objectives"]}`),
		}},
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentStatusTool,
			Description: "Read the actual coordinator and worker states for an existing delegation. A running status is not a completed task.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"coordination_id":{"type":"string"}},"required":["coordination_id"]}`),
		}},
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentWaitTool,
			Description: "Await actual completion of delegated workers within the current parent turn, with cancellation and a bounded timeout. Once terminal, review returned evidence and run parent build or tests before reporting success.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"coordination_id":{"type":"string"},"wait_seconds":{"type":"integer","minimum":1,"maximum":30}},"required":["coordination_id"]}`),
		}},
	)
}

func multiAgentToolFailure(code string, err error) chat.ToolResult {
	return chat.ToolResult{Status: "FAILED", Error: &chat.ToolError{Code: code, Message: err.Error(), Retryable: false}}
}

func multiAgentToolSuccess(v any) chat.ToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return multiAgentToolFailure("SERIALIZATION_FAILED", err)
	}
	return chat.ToolResult{Status: "SUCCESS", Output: data}
}

func (a *chatToolRuntimeAdapter) executeMultiAgentTool(ctx context.Context, name string, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	if !a.multiAgentAvailable(ctx, scope) {
		return multiAgentToolFailure("MULTI_AGENT_NOT_AUTHORIZED", fmt.Errorf("multi-agent tools are available only to an authorized, workspace-bound parent turn"))
	}
	ma := a.multiAgent
	if !json.Valid(input) || len(input) > 16384 {
		return multiAgentToolFailure("INVALID_INPUT", fmt.Errorf("multi-agent arguments are invalid or too large"))
	}
	if name == multiAgentDelegateTool {
		return ma.delegate(ctx, input, scope)
	}
	if name == multiAgentStatusTool {
		return ma.status(ctx, input, scope)
	}
	if name == multiAgentWaitTool {
		return ma.wait(ctx, input, scope)
	}
	return multiAgentToolFailure("UNKNOWN_TOOL", fmt.Errorf("unknown multi-agent tool"))
}

func (m *chatMultiAgentRuntime) authorizedParent(ctx context.Context, scope chat.SkillScope) (*interaction.InteractionRecord, error) {
	parent, found, err := m.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
	if err != nil {
		return nil, err
	}
	if !found || parent == nil {
		return nil, fmt.Errorf("active parent interaction is not registered for the current request")
	}
	resolved := parent.Scope.Normalize()
	if resolved.SpaceID != scope.SpaceID || resolved.ConversationID != scope.ConversationID ||
		resolved.CharacterID != scope.CharacterID || resolved.Source == "multi_agent" {
		return nil, fmt.Errorf("parent interaction scope does not match the authorized model turn")
	}
	if parent.IsTerminal() {
		return nil, fmt.Errorf("parent interaction has already finished")
	}
	return parent, nil
}

func (m *chatMultiAgentRuntime) delegate(ctx context.Context, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	key := scope.SpaceID + ":" + scope.RequestID
	m.mu.Lock()
	if m.dispatches == nil {
		m.dispatches = make(map[string]*multiAgentDispatchLock)
	}
	lock := m.dispatches[key]
	if lock == nil {
		lock = &multiAgentDispatchLock{}
		m.dispatches[key] = lock
	}
	lock.waiters++
	m.mu.Unlock()
	lock.mu.Lock()
	defer func() {
		lock.mu.Unlock()
		m.mu.Lock()
		lock.waiters--
		if lock.waiters == 0 {
			delete(m.dispatches, key)
		}
		m.mu.Unlock()
	}()
	return m.delegateOnce(ctx, input, scope)
}

func (m *chatMultiAgentRuntime) delegateOnce(ctx context.Context, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	parent, err := m.authorizedParent(ctx, scope)
	if err != nil {
		return multiAgentToolFailure("PARENT_SCOPE_INVALID", err)
	}
	if parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
		ref := parent.RecoveryDescriptor.MultiAgent
		return multiAgentToolSuccess(map[string]any{"coordinationId": ref.CoordinationID, "status": ref.Status, "alreadyExists": true, "assignmentRefs": ref.AssignmentRefs})
	}
	var request struct {
		Objectives []string `json:"objectives"`
		Strategy   string   `json:"strategy"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return multiAgentToolFailure("INVALID_INPUT", err)
	}
	if len(request.Objectives) < 2 || len(request.Objectives) > 4 {
		return multiAgentToolFailure("INVALID_OBJECTIVES", fmt.Errorf("multi-agent dispatch requires two to four objectives"))
	}
	refs := make([]interaction.AgentWorkerRef, 0, len(request.Objectives))
	assignments := make([]interaction.AssignmentObjective, 0, len(request.Objectives))
	for i, obj := range request.Objectives {
		obj = strings.TrimSpace(obj)
		if len(obj) < 8 || len(obj) > 2000 {
			return multiAgentToolFailure("INVALID_OBJECTIVES", fmt.Errorf("objective %d has invalid length", i))
		}
		refs = append(refs, interaction.AgentWorkerRef{WorkerID: fmt.Sprintf("worker-%d", i), CharacterID: scope.CharacterID, Role: "coding_worker"})
		assignments = append(assignments, interaction.AssignmentObjective{WorkerIndex: i, Objective: obj, ExpectedOutcome: "Deliver a verified, scoped result for this objective"})
	}
	strategy := interaction.CoordinationParallel
	if request.Strategy != "" {
		switch request.Strategy {
		case string(interaction.CoordinationParallel), string(interaction.CoordinationSequential):
			strategy = interaction.CoordinationStrategy(request.Strategy)
		default:
			return multiAgentToolFailure("INVALID_STRATEGY", fmt.Errorf("unsupported worker strategy"))
		}
	}
	goal := decision.NewGoal(decision.GoalCreateRequest{
		SpaceID: scope.SpaceID, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID,
		Description: "Workspace task delegated by " + scope.RequestID,
	}, time.Now().UTC())
	if err := m.goals.Register(goal); err != nil {
		return multiAgentToolFailure("GOAL_REGISTRATION_FAILED", err)
	}
	workspace := scope.ExecContext
	requestStart := interaction.StartCoordinationRequest{
		ParentInteractionID: parent.ID, ParentGoalID: goal.ID, ParentGoalRevision: goal.Revision,
		WorkerRefs: refs, Objectives: assignments, Strategy: strategy,
		CompletionPlan: interaction.CoordinationRequireAll, WorkspaceID: workspace.WorkspaceID,
		PermissionMode: scope.PermissionMode,
	}
	if workspace.Metadata != nil {
		if v, ok := workspace.Metadata["workspaceDeviceId"].(string); ok {
			requestStart.WorkspaceDeviceID = v
		}
		if v, ok := workspace.Metadata["workspaceName"].(string); ok {
			requestStart.WorkspaceName = v
		}
		if v, ok := workspace.Metadata["workspaceKind"].(string); ok {
			requestStart.WorkspaceKind = v
		}
		if v, ok := workspace.Metadata["workspaceRootUri"].(string); ok {
			requestStart.WorkspaceRootURI = v
		}
	}
	if workspace.RuntimeTarget != nil && requestStart.WorkspaceDeviceID == "" {
		requestStart.WorkspaceDeviceID = string(workspace.RuntimeTarget.DeviceID)
	}
	started, err := m.coordinator.Start(ctx, requestStart)
	if err != nil {
		return multiAgentToolFailure("MULTI_AGENT_DISPATCH_FAILED", err)
	}
	var snapshot interaction.CoordinationSnapshot
	for attempt := 0; attempt <= len(assignments); attempt++ {
		if err := m.coordinator.Reconcile(ctx, string(started.CoordinationID)); err != nil {
			return multiAgentToolFailure("MULTI_AGENT_RECONCILIATION_FAILED", err)
		}
		snapshot, err = m.coordinator.Snapshot(ctx, string(started.CoordinationID))
		if err != nil {
			return multiAgentToolFailure("MULTI_AGENT_SNAPSHOT_FAILED", err)
		}
		if snapshot.Status == interaction.CoordinationSucceeded ||
			snapshot.Status == interaction.CoordinationFailed ||
			snapshot.Status == interaction.CoordinationCancelled {
			break
		}
	}
	return multiAgentToolSuccess(map[string]any{
		"coordinationId": started.CoordinationID, "assignmentIds": started.AssignmentIDs,
		"status": snapshot.Status, "assignments": snapshot.Assignments,
		"verification": "requires_parent_review",
		"message":      "Worker outcomes reflect child interactions; validate artifacts and tests before claiming the parent task complete",
	})
}

func (m *chatMultiAgentRuntime) status(ctx context.Context, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	var request struct {
		CoordinationID string `json:"coordination_id"`
	}
	if err := json.Unmarshal(input, &request); err != nil || strings.TrimSpace(request.CoordinationID) == "" || len(request.CoordinationID) > 128 {
		return multiAgentToolFailure("INVALID_INPUT", fmt.Errorf("a valid coordination_id is required"))
	}
	snapshot, err := m.coordinator.Snapshot(ctx, request.CoordinationID)
	if err != nil {
		records, scanErr := m.tracker.ListByScope(ctx, interaction.InteractionScope{
			SpaceID: scope.SpaceID, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID,
		})
		if scanErr != nil {
			return multiAgentToolFailure("COORDINATION_RECOVERY_FAILED", scanErr)
		}
		for _, record := range records {
			if record == nil || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
				continue
			}
			resolved := record.Scope.Normalize()
			if resolved.SpaceID != scope.SpaceID || resolved.CharacterID != scope.CharacterID ||
				resolved.ConversationID != scope.ConversationID ||
				record.RecoveryDescriptor.MultiAgent.CoordinationID != request.CoordinationID {
				continue
			}
			if restoreErr := m.coordinator.RecoverCoordination(ctx, record); restoreErr != nil {
				return multiAgentToolFailure("COORDINATION_RECOVERY_FAILED", restoreErr)
			}
			snapshot, err = m.coordinator.Snapshot(ctx, request.CoordinationID)
			break
		}
		if err != nil {
			return multiAgentToolFailure("COORDINATION_NOT_FOUND", err)
		}
	}
	parent, found, err := m.tracker.Get(ctx, snapshot.ParentInteractionID)
	if err != nil {
		return multiAgentToolFailure("PARENT_LOOKUP_FAILED", err)
	}
	if !found || parent == nil {
		return multiAgentToolFailure("PARENT_NOT_FOUND", fmt.Errorf("delegation parent is not available"))
	}
	resolved := parent.Scope.Normalize()
	if resolved.SpaceID != scope.SpaceID || resolved.ConversationID != scope.ConversationID ||
		resolved.CharacterID != scope.CharacterID {
		return multiAgentToolFailure("COORDINATION_NOT_AUTHORIZED", fmt.Errorf("coordination is outside the current conversation scope"))
	}
	if err := m.coordinator.Reconcile(ctx, request.CoordinationID); err != nil {
		return multiAgentToolFailure("COORDINATION_RECONCILIATION_FAILED", err)
	}
	snapshot, err = m.coordinator.Snapshot(ctx, request.CoordinationID)
	if err != nil {
		return multiAgentToolFailure("COORDINATION_NOT_FOUND", err)
	}
	return multiAgentToolSuccess(snapshot)
}
