package permission

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type EvaluationRequestBuilder struct {
	request PermissionEvaluationRequest
}

func NewEvaluationRequest(subject PermissionSubject, requirements []PermissionRequirement) *EvaluationRequestBuilder {
	return &EvaluationRequestBuilder{
		request: PermissionEvaluationRequest{
			Subject:      subject,
			Requirements: requirements,
		},
	}
}

func (b *EvaluationRequestBuilder) WithInvocationID(id string) *EvaluationRequestBuilder {
	b.request.InvocationID = id
	return b
}

func (b *EvaluationRequestBuilder) WithInput(input []byte) *EvaluationRequestBuilder {
	b.request.Input = input
	return b
}

func (b *EvaluationRequestBuilder) WithRiskLevel(level string) *EvaluationRequestBuilder {
	b.request.RiskLevel = level
	return b
}

func (b *EvaluationRequestBuilder) WithIsBackground(isBackground bool) *EvaluationRequestBuilder {
	b.request.IsBackground = isBackground
	return b
}

func (b *EvaluationRequestBuilder) WithScopeSnapshotID(id string) *EvaluationRequestBuilder {
	b.request.ScopeSnapshotID = id
	return b
}

func (b *EvaluationRequestBuilder) WithApprovalMode(mode string) *EvaluationRequestBuilder {
	b.request.ApprovalMode = mode
	return b
}

func (b *EvaluationRequestBuilder) WithGeneration(generation int) *EvaluationRequestBuilder {
	b.request.Generation = generation
	return b
}

func (b *EvaluationRequestBuilder) WithExecutionContext(ctx PermissionExecutionContext) *EvaluationRequestBuilder {
	b.request.ExecutionContext = ctx
	return b
}

func (b *EvaluationRequestBuilder) WithApprovalRecordID(id string) *EvaluationRequestBuilder {
	b.request.ApprovalRecordID = id
	return b
}

func (b *EvaluationRequestBuilder) Build() PermissionEvaluationRequest {
	return b.request
}

func BuildRequirements(tool capability.ToolDefinition, scope PermissionScope) []PermissionRequirement {
	reqs := make([]PermissionRequirement, 0, len(tool.Permissions))
	for _, p := range tool.Permissions {
		reqs = append(reqs, PermissionRequirement{
			PermissionID: p.Capability,
			Scope:        scope,
		})
	}
	return reqs
}

func BuildRequirementsFromIDs(permissionIDs []string, scope PermissionScope) []PermissionRequirement {
	reqs := make([]PermissionRequirement, 0, len(permissionIDs))
	for _, id := range permissionIDs {
		reqs = append(reqs, PermissionRequirement{
			PermissionID: id,
			Scope:        scope,
		})
	}
	return reqs
}

func BuildEvaluationRequestFromInvocation(subject PermissionSubject, requirements []PermissionRequirement, inv capability.ToolInvocationContext, riskLevel string) PermissionEvaluationRequest {
	return NewEvaluationRequest(subject, requirements).
		WithInvocationID(inv.InvocationID).
		WithRiskLevel(riskLevel).
		WithIsBackground(inv.IsBackground).
		WithScopeSnapshotID(inv.ScopeSnapshotID).
		WithApprovalMode(string(inv.ApprovalMode)).
		WithGeneration(int(inv.Generation)).
		WithExecutionContext(ExecutionContextFromInvocation(inv)).
		Build()
}

func ResolveScopeForInvocation(inv capability.ToolInvocationContext) PermissionScope {
	if inv.CharacterID != "" {
		return ScopeForCharacter(inv.CharacterID)
	}
	if inv.ConversationID != "" {
		return ScopeForConversation(inv.ConversationID)
	}
	return ScopeGlobalOnly()
}
