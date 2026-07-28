package host_api

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type BrokerPermissionChecker struct {
	Broker permission.PermissionBroker
}

func NewBrokerPermissionChecker(broker permission.PermissionBroker) *BrokerPermissionChecker {
	return &BrokerPermissionChecker{Broker: broker}
}

func (c *BrokerPermissionChecker) Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, reqs []PermissionRequirement) error {
	if c == nil || c.Broker == nil {
		return ErrPermissionDenied
	}

	if len(reqs) == 0 {
		return nil
	}

	subject := PermissionSubjectFromIdentity(identity)
	permReqs := make([]permission.PermissionRequirement, 0, len(reqs))
	for _, r := range reqs {
		if r.Name == "" {
			return fmt.Errorf("%w: empty permission name", ErrPermissionDenied)
		}
		permReqs = append(permReqs, permission.PermissionRequirement{
			PermissionID: r.Name,
			Scope:        permission.ScopeForExtension(subject.ExtensionID),
		})
	}

	result := c.Broker.Evaluate(ctx, permission.PermissionEvaluationRequest{
		Subject:      subject,
		Requirements: permReqs,
	})

	if result.Decision != permission.DecisionAllow {
		return fmt.Errorf("%w: decision=%s missing=%d reasons=%v", ErrPermissionDenied, result.Decision, len(result.Missing), result.Reasons)
	}
	return nil
}
