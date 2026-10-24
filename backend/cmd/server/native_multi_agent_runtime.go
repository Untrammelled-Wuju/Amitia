package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

const multiAgentDelegateTool = "harness_multi_agent_delegate"
const multiAgentStatusTool = "harness_multi_agent_status"
const multiAgentWaitTool = "harness_multi_agent_wait"

type chatMultiAgentRuntime struct {
	coordinator      *interaction.MultiAgentCoordinator
	tracker          interaction.InteractionTracker
	goals            *decision.GoalRegistry
	worktreeReviewer codingWorktreeReviewer
	terminalReader   codingTaskTerminalReader
	mu               sync.Mutex
	dispatches       map[string]*multiAgentDispatchLock
}

type multiAgentDispatchLock struct {
	mu      sync.Mutex
	waiters int
}

func isChatMultiAgentTool(name string) bool {
	return name == multiAgentDelegateTool || name == multiAgentStatusTool || name == multiAgentWaitTool
}

func (m *chatMultiAgentRuntime) available(ctx context.Context, scope chat.SkillScope) bool {
	if m == nil || m.coordinator == nil ||
		m.tracker == nil || m.goals == nil || scope.ExecContext == nil {
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
	parent, found, err := m.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
	if err != nil || !found || parent == nil || parent.IsTerminal() || !m.ownedParentAuthorized(ctx, parent) {
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

func (m *chatMultiAgentRuntime) ModelTools(ctx context.Context, scope chat.SkillScope) []tool.Tool {
	if !m.available(ctx, scope) || coordination.ValidateCurrent(ctx) != nil {
		return nil
	}
	return []tool.Tool{
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentDelegateTool,
			Description: "Delegate two to four coding subtasks within the authorized workspace. Optional dependencies is an array aligned to objectives; each row contains zero-based prerequisite objective indices, with cycles and self-dependencies forbidden. Optional deadline_minutes limits the entire coordination to 1-240 minutes. Optional max_tool_calls limits the total number of actual authorized tool invocations across all child workers (1-256); reservations are durable and uncertain effects remain charged. Optional max_total_tokens sets the parent total model-token allowance (1-100000000); the full configured context window is reserved before each model invocation, with unused tokens released only after trusted usage reporting. Calls without a configured context window are blocked when this limit is enabled. Optional max_cost_microusd limits the shared configured-rate cost allowance (millionths of USD) across children; it fails closed unless the model has an operator-maintained rate-ceiling version and input/output rates. This is NOT verification of the provider's final invoice. Returns task IDs, not proof of completion. Do not dispatch another batch until the first finishes.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"objectives":{"type":"array","minItems":2,"maxItems":4,"items":{"type":"string","minLength":8,"maxLength":2000}},"strategy":{"type":"string","enum":["parallel","sequential"]},"dependencies":{"type":"array","minItems":2,"maxItems":4,"items":{"type":"array","items":{"type":"integer","minimum":0,"maximum":3},"uniqueItems":true}},"deadline_minutes":{"type":"integer","minimum":1,"maximum":240},"max_tool_calls":{"type":"integer","minimum":1,"maximum":256},"max_total_tokens":{"type":"integer","minimum":1,"maximum":100000000},"max_cost_microusd":{"type":"integer","minimum":1,"maximum":100000000000}},"required":["objectives"],"additionalProperties":false}`),
		}},
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentStatusTool,
			Description: "Read coordinator and coding worker states, including worktree evidence, trusted terminal metadata and verificationGate. The verificationSummary only reflects scoped evidence; worker succeeded is not verified test success. Never claim completion without parent verification.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"coordination_id":{"type":"string"}},"required":["coordination_id"]}`),
		}},
		tool.Tool{Type: "function", Function: tool.Function{
			Name:        multiAgentWaitTool,
			Description: "Await actual worker completion with bounded timeout. After terminal, inspect each worktreeReview.verificationGate and verificationSummary. Worker success and exit code zero are not evidence of tested or merged code. Manual approval remains required.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"coordination_id":{"type":"string"},"wait_seconds":{"type":"integer","minimum":1,"maximum":30}},"required":["coordination_id"]}`),
		}},
	}
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

func (m *chatMultiAgentRuntime) Execute(ctx context.Context, name string, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return multiAgentToolFailure("MULTI_AGENT_NOT_AUTHORIZED", err)
	}
	if !m.available(ctx, scope) {
		return multiAgentToolFailure("MULTI_AGENT_NOT_AUTHORIZED", fmt.Errorf("multi-agent tools are available only to an authorized, workspace-bound parent turn"))
	}
	if !json.Valid(input) || len(input) > 16384 {
		return multiAgentToolFailure("INVALID_INPUT", fmt.Errorf("multi-agent arguments are invalid or too large"))
	}
	if name == multiAgentDelegateTool {
		return m.delegate(ctx, input, scope)
	}
	if name == multiAgentStatusTool {
		return m.status(ctx, input, scope)
	}
	if name == multiAgentWaitTool {
		return m.wait(ctx, input, scope)
	}
	return multiAgentToolFailure("UNKNOWN_TOOL", fmt.Errorf("unknown multi-agent tool"))
}

func (m *chatMultiAgentRuntime) authorizedParent(ctx context.Context, scope chat.SkillScope) (*interaction.InteractionRecord, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
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
	if !m.ownedParentAuthorized(ctx, parent) {
		return nil, coordination.ErrWrongOwner
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
	request, err := parseMultiAgentDelegateArguments(input)
	if err != nil {
		return multiAgentToolFailure("INVALID_INPUT", err)
	}
	if len(request.Objectives) < 2 || len(request.Objectives) > 4 {
		return multiAgentToolFailure("INVALID_OBJECTIVES", fmt.Errorf("multi-agent dispatch requires two to four objectives"))
	}
	if err := validateMultiAgentDeadlineMinutes(request.DeadlineMinutes); err != nil {
		return multiAgentToolFailure("INVALID_DEADLINE", err)
	}
	if request.Dependencies != nil && len(request.Dependencies) != len(request.Objectives) {
		return multiAgentToolFailure("INVALID_DEPENDENCIES", fmt.Errorf("dependencies must contain one row per objective"))
	}
	refs := make([]interaction.AgentWorkerRef, 0, len(request.Objectives))
	assignments := make([]interaction.AssignmentObjective, 0, len(request.Objectives))
	for i, obj := range request.Objectives {
		obj = strings.TrimSpace(obj)
		request.Objectives[i] = obj
		if len(obj) < 8 || len(obj) > 2000 {
			return multiAgentToolFailure("INVALID_OBJECTIVES", fmt.Errorf("objective %d has invalid length", i))
		}
		refs = append(refs, interaction.AgentWorkerRef{WorkerID: fmt.Sprintf("worker-%d", i), CharacterID: scope.CharacterID, Role: "coding_worker"})
		assignment := interaction.AssignmentObjective{WorkerIndex: i, Objective: obj, ExpectedOutcome: "Deliver a verified, scoped result for this objective"}
		if request.Dependencies != nil {
			for _, predecessor := range request.Dependencies[i] {
				if predecessor < 0 || predecessor >= len(request.Objectives) || predecessor == i {
					return multiAgentToolFailure("INVALID_DEPENDENCIES", fmt.Errorf("invalid predecessor %d for objective %d", predecessor, i))
				}
				assignment.Dependencies = append(assignment.Dependencies, fmt.Sprintf("index:%d", predecessor))
			}
		}
		assignments = append(assignments, assignment)
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
	request.Strategy = string(strategy)
	canonical, _ := json.Marshal(struct {
		Objectives      []string `json:"objectives"`
		Strategy        string   `json:"strategy"`
		WorkspaceID     string   `json:"workspaceId"`
		PermissionMode  string   `json:"permissionMode"`
		Dependencies    [][]int  `json:"dependencies,omitempty"`
		DeadlineMinutes *int     `json:"deadline_minutes,omitempty"`
		MaxToolCalls    *int     `json:"max_tool_calls,omitempty"`
		MaxTotalTokens  *int64   `json:"max_total_tokens,omitempty"`
		MaxCostMicroUSD *int64   `json:"max_cost_microusd,omitempty"`
	}{request.Objectives, request.Strategy, scope.ExecContext.WorkspaceID, scope.PermissionMode, request.Dependencies, request.DeadlineMinutes, request.MaxToolCalls, request.MaxTotalTokens, request.MaxCostMicroUSD})
	digest := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(digest[:])
	if parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
		ref := parent.RecoveryDescriptor.MultiAgent
		if ref.DispatchFingerprint != fingerprint {
			return multiAgentToolFailure("MULTI_AGENT_DISPATCH_CONFLICT", fmt.Errorf("parent request already has a delegation with different arguments"))
		}
		return multiAgentToolSuccess(map[string]any{"coordinationId": ref.CoordinationID, "status": ref.Status, "alreadyExists": true, "assignmentRefs": ref.AssignmentRefs})
	}
	if err := interaction.ValidateAssignmentDependencyGraph(assignments); err != nil {
		return multiAgentToolFailure("INVALID_DEPENDENCIES", err)
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
		PermissionMode: scope.PermissionMode, DispatchFingerprint: fingerprint,
		MaxToolCalls: request.MaxToolCalls, MaxTotalTokens: request.MaxTotalTokens, MaxCostMicroUSD: request.MaxCostMicroUSD,
	}
	if owned, ok := coordination.FromContext(ctx); ok {
		if owned.SpaceID != scope.SpaceID || owned.RoleID != scope.CharacterID || owned.CoreID == "" || owned.ResourceOwnerID == "" {
			return multiAgentToolFailure("PARENT_SCOPE_INVALID", coordination.ErrWrongOwner)
		}
		requestStart.ExecutionCoreID, requestStart.ResourceOwnerID = owned.CoreID, owned.ResourceOwnerID
		taskScope := ownedTaskExecutionScope(owned)
		requestStart.OwnedScope = &taskScope
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
	if request.DeadlineMinutes != nil {
		deadline := multiAgentDeadlineFromMinutes(time.Now().UTC(), *request.DeadlineMinutes)
		requestStart.DeadlineAt = &deadline
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
	m.attachCodingWorktreeEvidence(ctx, scope, &snapshot)
	return multiAgentToolSuccess(map[string]any{
		"coordinationId": started.CoordinationID, "assignmentIds": started.AssignmentIDs,
		"status": snapshot.Status, "assignments": snapshot.Assignments,
		"verificationSummary": snapshot.VerificationSummary,
		"verification":        "requires_parent_review",
		"message":             "Worker outcomes reflect child interactions; validate artifacts and tests before claiming the parent task complete",
	})
}

func ownedTaskExecutionScope(parent coordination.ExecutionScope) coordination.ExecutionScope {
	parent.ContinuityRoleID = ""
	return parent
}

func (m *chatMultiAgentRuntime) status(ctx context.Context, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return multiAgentToolFailure("COORDINATION_NOT_AUTHORIZED", err)
	}
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
				record.RecoveryDescriptor.MultiAgent.CoordinationID != request.CoordinationID ||
				!m.ownedParentAuthorized(ctx, record) {
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
		resolved.CharacterID != scope.CharacterID || !m.ownedParentAuthorized(ctx, parent) {
		return multiAgentToolFailure("COORDINATION_NOT_AUTHORIZED", fmt.Errorf("coordination is outside the current conversation scope"))
	}
	if err := m.coordinator.Reconcile(ctx, request.CoordinationID); err != nil {
		return multiAgentToolFailure("COORDINATION_RECONCILIATION_FAILED", err)
	}
	snapshot, err = m.coordinator.Snapshot(ctx, request.CoordinationID)
	if err != nil {
		return multiAgentToolFailure("COORDINATION_NOT_FOUND", err)
	}
	m.attachCodingWorktreeEvidence(ctx, scope, &snapshot)
	return multiAgentToolSuccess(snapshot)
}

func (m *chatMultiAgentRuntime) ownedParentAuthorized(ctx context.Context, parent *interaction.InteractionRecord) bool {
	owned, present := coordination.FromContext(ctx)
	if parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.OwnedTurn != nil {
		bound := parent.RecoveryDescriptor.OwnedTurn.Scope
		return present && owned.SpaceID == bound.SpaceID && owned.RoleID == bound.RoleID &&
			owned.CoreID != "" && owned.CoreID == bound.CoreID && owned.ResourceOwnerID != "" &&
			owned.ResourceOwnerID == bound.ResourceOwnerID && owned.RoleOwnerID == bound.RoleOwnerID &&
			owned.AuthorizationRealm == bound.AuthorizationRealm
	}
	if parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
		bound := parent.RecoveryDescriptor.MultiAgent
		if bound.ExecutionCoreID != "" || bound.ResourceOwnerID != "" {
			return present && owned.CoreID == bound.ExecutionCoreID && owned.ResourceOwnerID == bound.ResourceOwnerID
		}
	}
	return !present || (owned.SpaceID == parent.Scope.SpaceID && owned.RoleID == parent.Scope.CharacterID && owned.CoreID != "" && owned.ResourceOwnerID != "")
}
