package hook

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type RuntimeBridge interface {
	Invoke(ctx context.Context, contrib HookContributionDefinition, input HookInvocationInput) (HookResult, error)
	IsReady(ctx context.Context, contrib HookContributionDefinition) bool
}

type PermissionChecker interface {
	Check(ctx context.Context, extensionID string, requirements []permission.PermissionRequirement, invocationID string) (bool, string)
}

type ScopeChecker interface {
	Check(ctx context.Context, req scope.ScopeEvaluationRequest) (bool, string)
}

type DependencyChecker interface {
	Check(ctx context.Context, requirements []DependencyRequirement) (bool, string)
}

type TraceRecorder interface {
	RecordInvocation(ctx context.Context, exec HookExecution, inputHash, resultHash string)
	RecordMutation(ctx context.Context, invocationID string, op MutationOperation, beforeHash, afterHash string, applied bool, conflict bool)
	RecordPipeline(ctx context.Context, result PipelineResult)
	RecordCircuit(ctx context.Context, contributionID string, stats CircuitStats)
}

type ContributionStore interface {
	ListByHookPoint(ctx context.Context, hookPointID string) ([]HookContributionDefinition, error)
	Get(ctx context.Context, contributionID string) (HookContributionDefinition, error)
	Register(ctx context.Context, contrib HookContributionDefinition) error
	Unregister(ctx context.Context, contributionID string) error
	SetEnabled(ctx context.Context, contributionID string, enabled bool) error
	ListByExtension(ctx context.Context, extensionID string) ([]HookContributionDefinition, error)
	List(ctx context.Context) ([]HookContributionDefinition, error)
}

type NopTraceRecorder struct{}

func (NopTraceRecorder) RecordInvocation(_ context.Context, _ HookExecution, _, _ string) {}
func (NopTraceRecorder) RecordMutation(_ context.Context, _ string, _ MutationOperation, _, _ string, _, _ bool) {
}
func (NopTraceRecorder) RecordPipeline(_ context.Context, _ PipelineResult)        {}
func (NopTraceRecorder) RecordCircuit(_ context.Context, _ string, _ CircuitStats) {}

type NopPermissionChecker struct{}

func (NopPermissionChecker) Check(_ context.Context, _ string, _ []permission.PermissionRequirement, _ string) (bool, string) {
	return true, ""
}

type NopScopeChecker struct{}

func (NopScopeChecker) Check(_ context.Context, _ scope.ScopeEvaluationRequest) (bool, string) {
	return true, ""
}

type NopDependencyChecker struct{}

func (NopDependencyChecker) Check(_ context.Context, _ []DependencyRequirement) (bool, string) {
	return true, ""
}
