package schedule

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type BrokerPermissionChecker struct {
	Broker permission.PermissionBroker
	Store  ScheduleStore
}

func NewBrokerPermissionChecker(broker permission.PermissionBroker, store ScheduleStore) *BrokerPermissionChecker {
	return &BrokerPermissionChecker{Broker: broker, Store: store}
}

func (c *BrokerPermissionChecker) CheckPermission(ctx context.Context, scheduleID string, requirements []PermissionRequirement, isBackground bool) (bool, string, error) {
	if c == nil {
		return false, "permission checker not configured", nil
	}
	if c.Broker == nil {
		return false, "permission broker not configured", nil
	}
	if len(requirements) == 0 {
		return true, "", nil
	}

	def, err := c.Store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return false, "schedule definition not found", nil
	}

	subject := permission.SubjectForExtension(def.ExtensionID)
	if def.ModuleID != "" {
		subject = permission.PermissionSubject{
			Type:        permission.SubjectModule,
			ID:          def.ModuleID,
			ExtensionID: def.ExtensionID,
			ModuleID:    def.ModuleID,
		}
	}

	permReqs := make([]permission.PermissionRequirement, 0, len(requirements))
	for _, r := range requirements {
		if r.PermissionID == "" {
			continue
		}
		permReqs = append(permReqs, permission.PermissionRequirement{
			PermissionID: r.PermissionID,
			Scope:        permission.ScopeForExtension(def.ExtensionID),
			Optional:     r.Optional,
		})
	}

	if len(permReqs) == 0 {
		return true, "", nil
	}

	result := c.Broker.Evaluate(ctx, permission.PermissionEvaluationRequest{
		Subject:      subject,
		Requirements: permReqs,
		IsBackground: isBackground,
	})

	if result.Decision != permission.DecisionAllow &&
		result.Decision != permission.DecisionAllowPersistent &&
		result.Decision != permission.DecisionAllowOnce &&
		result.Decision != permission.DecisionAllowSession {
		reason := fmt.Sprintf("decision=%s missing=%d", result.Decision, len(result.Missing))
		if len(result.Reasons) > 0 {
			reason = reason + " reasons=" + fmt.Sprint(result.Reasons)
		}
		return false, reason, nil
	}
	return true, "", nil
}

var _ PermissionChecker = (*BrokerPermissionChecker)(nil)
