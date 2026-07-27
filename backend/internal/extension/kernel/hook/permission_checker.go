package hook

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type PermissionBrokerChecker struct {
	Broker permission.PermissionBroker
}

func NewPermissionBrokerChecker(broker permission.PermissionBroker) *PermissionBrokerChecker {
	return &PermissionBrokerChecker{Broker: broker}
}

func (c *PermissionBrokerChecker) Check(ctx context.Context, extensionID string, requirements []permission.PermissionRequirement, invocationID string) (bool, string) {
	if c.Broker == nil {
		return true, ""
	}
	if len(requirements) == 0 {
		return true, ""
	}

	req := permission.PermissionEvaluationRequest{
		Subject: permission.PermissionSubject{
			Type: permission.SubjectExtension,
			ID:   extensionID,
		},
		Requirements:  requirements,
		InvocationID: invocationID,
		IsBackground:  false,
	}

	result := c.Broker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow {
		return true, ""
	}

	reason := "denied"
	if len(result.Reasons) > 0 {
		reason = result.Reasons[0].Detail
		if reason == "" {
			reason = result.Reasons[0].Code
		}
	}
	return false, reason
}
