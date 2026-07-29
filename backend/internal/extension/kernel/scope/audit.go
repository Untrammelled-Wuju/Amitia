package scope

import (
	"context"
)

type ScopeAuditor interface {
	RecordBindingCreated(ctx context.Context, binding ScopeBinding)
	RecordBindingDeleted(ctx context.Context, binding ScopeBinding)
	RecordBindingRevoked(ctx context.Context, binding ScopeBinding)
	RecordEvaluation(ctx context.Context, req ScopeEvaluationRequest, decision ScopeDecision)
	RecordInvalidation(ctx context.Context, filter ScopeInvalidationFilter, count int)
}

type NoOpScopeAuditor struct{}

func (NoOpScopeAuditor) RecordBindingCreated(ctx context.Context, binding ScopeBinding) {}
func (NoOpScopeAuditor) RecordBindingDeleted(ctx context.Context, binding ScopeBinding) {}
func (NoOpScopeAuditor) RecordBindingRevoked(ctx context.Context, binding ScopeBinding) {}
func (NoOpScopeAuditor) RecordEvaluation(ctx context.Context, req ScopeEvaluationRequest, decision ScopeDecision) {
}
func (NoOpScopeAuditor) RecordInvalidation(ctx context.Context, filter ScopeInvalidationFilter, count int) {
}
