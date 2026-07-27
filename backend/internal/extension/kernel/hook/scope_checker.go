package hook

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type ScopeManagerChecker struct {
	Manager scope.ScopeManager
}

func NewScopeManagerChecker(manager scope.ScopeManager) *ScopeManagerChecker {
	return &ScopeManagerChecker{Manager: manager}
}

func (c *ScopeManagerChecker) Check(ctx context.Context, req scope.ScopeEvaluationRequest) (bool, string) {
	if c.Manager == nil {
		return true, ""
	}

	decision := c.Manager.Evaluate(ctx, req)
	if decision.Allowed {
		return true, ""
	}

	reason := "scope denied"
	if len(decision.Reasons) > 0 {
		reason = decision.Reasons[0].Code
	}
	return false, reason
}
