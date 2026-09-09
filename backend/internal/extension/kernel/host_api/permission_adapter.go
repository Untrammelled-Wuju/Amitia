package host_api

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type permissionEvaluationContextKey struct{}

func withPermissionEvaluationContext(ctx context.Context, request CallRequest) context.Context {
	return context.WithValue(ctx, permissionEvaluationContextKey{}, request)
}

func permissionEvaluationRequest(ctx context.Context) (CallRequest, bool) {
	request, ok := ctx.Value(permissionEvaluationContextKey{}).(CallRequest)
	return request, ok
}

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

	evaluation := permission.PermissionEvaluationRequest{
		Subject:      subject,
		Requirements: permReqs,
	}
	if request, ok := permissionEvaluationRequest(ctx); ok {
		evaluation.InvocationID = request.InvocationID
		evaluation.ScopeSnapshotID = request.ScopeSnapshotID
		evaluation.ApprovalRecordID = request.ApprovalRecordID
		evaluation.ExecutionContext = request.ExecutionContext
	}
	result := c.Broker.Evaluate(ctx, evaluation)

	if result.Decision != permission.DecisionAllow {
		return fmt.Errorf("%w: decision=%s missing=%d reasons=%v", ErrPermissionDenied, result.Decision, len(result.Missing), result.Reasons)
	}
	return nil
}
